/*
Copyright 2020 Google LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"

	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clients"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/config"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/liveness"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/metrics"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/sslcertificatemanager"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/state"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/sync"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/flags"
	ingressutils "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/ingress"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/queue"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/random"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/types"
)

type params struct {
	clients        *clients.Clients
	config         *config.Config
	metrics        metrics.Interface
	healthCheck    *liveness.HealthCheck
	resyncInterval time.Duration
	state          state.Interface
	sync           sync.Interface
}

type controller struct {
	clients                       *clients.Clients
	ingressQueue                  workqueue.RateLimitingInterface
	ingressResyncQueue            workqueue.RateLimitingInterface
	managedCertificateQueue       workqueue.RateLimitingInterface
	managedCertificateResyncQueue workqueue.RateLimitingInterface
	metrics                       metrics.Interface
	healthCheck                   *liveness.HealthCheck
	resyncInterval                time.Duration
	state                         state.Interface
	sync                          sync.Interface
}

func NewParams(ctx context.Context, clients *clients.Clients, config *config.Config) *params {
	healthCheck := liveness.NewHealthCheck(flags.F.HealthCheckInterval,
		2*flags.F.ResyncInterval, 2*flags.F.ResyncInterval)
	metrics := metrics.New(config)
	state := state.New(ctx, clients.ConfigMap)
	ssl := sslcertificatemanager.New(clients.Event, metrics, clients.Ssl, state)
	random := random.New(config.SslCertificateNamePrefix)

	return &params{
		clients:        clients,
		config:         config,
		metrics:        metrics,
		healthCheck:    healthCheck,
		resyncInterval: flags.F.ResyncInterval,
		state:          state,
		sync: sync.New(config, clients.Event, clients.Ingress,
			clients.ManagedCertificate, metrics, random, ssl, state),
	}
}

func New(ctx context.Context, p *params) *controller {
	return &controller{
		clients:     p.clients,
		metrics:     p.metrics,
		healthCheck: p.healthCheck,
		ingressQueue: workqueue.NewNamedRateLimitingQueue(
			workqueue.DefaultControllerRateLimiter(), "ingressQueue"),
		ingressResyncQueue: workqueue.NewNamedRateLimitingQueue(
			workqueue.DefaultControllerRateLimiter(), "ingressResyncQueue"),
		managedCertificateQueue: workqueue.NewNamedRateLimitingQueue(
			workqueue.DefaultControllerRateLimiter(), "managedCertificateQueue"),
		managedCertificateResyncQueue: workqueue.NewNamedRateLimitingQueue(
			workqueue.DefaultControllerRateLimiter(), "managedCertificateResyncQueue"),
		resyncInterval: p.resyncInterval,
		state:          p.state,
		sync:           p.sync,
	}
}

func (c *controller) Run(ctx context.Context, healthCheckAddress string) error {
	defer runtime.HandleCrash()
	defer c.ingressQueue.ShutDown()
	defer c.ingressResyncQueue.ShutDown()
	defer c.managedCertificateQueue.ShutDown()
	defer c.managedCertificateResyncQueue.ShutDown()
	defer c.healthCheck.Stop()

	klog.Info("Controller.Run()")

	klog.Info("Start reporting metrics")
	go c.metrics.Start(flags.F.PrometheusAddress)

	klog.Info("Start liveness probe health checks")
	c.healthCheck.Start(healthCheckAddress, flags.F.HealthCheckPath)

	c.clients.Run(ctx, c.ingressQueue, c.managedCertificateQueue)

	klog.Info("Waiting for cache sync")
	cacheCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if !cache.WaitForCacheSync(cacheCtx.Done(), c.clients.HasSynced) {
		return fmt.Errorf("Timed out waiting for cache sync")
	}
	klog.Info("Cache synced")

	go wait.Until(
		func() { c.processNext(ctx, c.ingressQueue, liveness.Undefined, c.sync.Ingress) },
		time.Second, ctx.Done())
	go wait.Until(
		func() { c.processNext(ctx, c.ingressResyncQueue, liveness.IngressResyncProcess, c.sync.Ingress) },
		time.Second, ctx.Done())
	go wait.Until(
		func() { c.processNext(ctx, c.managedCertificateQueue, liveness.Undefined, c.sync.ManagedCertificate) },
		time.Second, ctx.Done())
	go wait.Until(
		func() {
			c.processNext(ctx, c.managedCertificateResyncQueue, liveness.McrtResyncProcess, c.sync.ManagedCertificate)
		},
		time.Second, ctx.Done())
	go wait.Until(func() { c.synchronizeAll(ctx) }, c.resyncInterval, ctx.Done())
	go wait.Until(func() { c.reportMetrics() }, time.Minute, ctx.Done())

	klog.Info("Waiting for stop signal or error")

	<-ctx.Done()
	klog.Info("Received stop signal, shutting down")
	return nil
}

func (c *controller) synchronizeAll(ctx context.Context) {
	// TODO(b/204546048): Add a metric to measure how long syncAll takes.
	// loopStart := time.Now()
	// metrics.UpdateLastTime(metrics.Main, loopStart)
	c.healthCheck.UpdateLastActivity(liveness.SynchronizeAll, time.Now())
	ingressScheduled := false
	mcrtScheduled := false

	if ingresses, err := c.clients.Ingress.List(); err != nil {
		runtime.HandleError(err)
	} else {
		for _, ingress := range ingresses {
			if !ingressutils.IsGKE(ingress) {
				klog.Infof("Skipping non-GKE Ingress %s/%s: %v",
					ingress.Namespace, ingress.Name, *ingress)
			} else {
				queue.Add(c.ingressResyncQueue, ingress)
				ingressScheduled = true
			}
		}
	}

	if managedCertificates, err := c.clients.ManagedCertificate.List(); err != nil {
		runtime.HandleError(err)
	} else {
		for _, managedCertificate := range managedCertificates {
			queue.Add(c.managedCertificateResyncQueue, managedCertificate)
			mcrtScheduled = true
		}
	}

	for id := range c.state.List() {
		queue.AddId(c.managedCertificateResyncQueue, id)
		mcrtScheduled = true
	}

	c.healthCheck.UpdateLastSuccessSync(time.Now(), ingressScheduled, mcrtScheduled)
	// TODO(b/204546048): Add a metric to measure how long syncAll takes.
	// metrics.UpdateDurationFromStart(metrics.Main, loopStart)
}

func (c *controller) reportMetrics() {
	managedCertificates, err := c.clients.ManagedCertificate.List()
	if err != nil {
		runtime.HandleError(err)
		return
	}

	statuses := make(map[string]int, 0)
	for _, mcrt := range managedCertificates {
		statuses[mcrt.Status.CertificateStatus]++
	}

	c.metrics.ObserveManagedCertificatesStatuses(statuses)

	c.metrics.ObserveIngressHighPriorityQueueLength(c.ingressQueue.Len())
	c.metrics.ObserveIngressLowPriorityQueueLength(c.ingressResyncQueue.Len())
	c.metrics.ObserveManagedCertificateHighPriorityQueueLength(c.managedCertificateQueue.Len())
	c.metrics.ObserveManagedCertificateLowPriorityQueueLength(c.managedCertificateResyncQueue.Len())
}

func (c *controller) processNext(ctx context.Context, queue workqueue.RateLimitingInterface,
	activityName liveness.ActivityName, handle func(ctx context.Context, id types.Id) error) {

	obj, shutdown := queue.Get()

	if shutdown {
		return
	}

	go func() {
		defer queue.Done(obj)

		key, ok := obj.(string)
		if !ok {
			queue.Forget(obj)
			runtime.HandleError(fmt.Errorf("Expected string in queue but got %T", obj))
			return
		}

		ctx, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()

		namespace, name, err := cache.SplitMetaNamespaceKey(key)
		if err != nil {
			runtime.HandleError(err)
			return
		}

		err = handle(ctx, types.NewId(namespace, name))
		if err == nil {
			if activityName != liveness.Undefined {
				c.healthCheck.UpdateLastActivity(activityName, time.Now())
			}
			queue.Forget(obj)
			return
		}

		queue.AddRateLimited(obj)
		runtime.HandleError(err)
	}()
}
