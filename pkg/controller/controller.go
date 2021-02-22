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
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/metrics"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/sslcertificatemanager"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/state"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/sync"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/flags"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/queue"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/random"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/types"
)

type controller struct {
	clients                 *clients.Clients
	metrics                 metrics.Interface
	ingressQueue            workqueue.RateLimitingInterface
	managedCertificateQueue workqueue.RateLimitingInterface
	state                   state.Interface
	sync                    sync.Interface
}

func New(ctx context.Context, config *config.Config, clients *clients.Clients) *controller {
	metrics := metrics.New(config)
	state := state.New(ctx, clients.ConfigMap)
	ssl := sslcertificatemanager.New(clients.Event, metrics, clients.Ssl, state)
	random := random.New(config.SslCertificateNamePrefix)

	return &controller{
		clients:                 clients,
		metrics:                 metrics,
		ingressQueue:            workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "ingressQueue"),
		managedCertificateQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "managedCertificateQueue"),
		state:                   state,
		sync:                    sync.New(config, clients.Event, clients.Ingress, clients.ManagedCertificate, metrics, random, ssl, state),
	}
}

func (c *controller) Run(ctx context.Context) error {
	defer runtime.HandleCrash()
	defer c.ingressQueue.ShutDown()
	defer c.managedCertificateQueue.ShutDown()

	klog.Info("Controller.Run()")

	klog.Info("Start reporting metrics")
	go c.metrics.Start(flags.F.PrometheusAddress)

	c.clients.Run(ctx, c.ingressQueue, c.managedCertificateQueue)

	klog.Info("Waiting for cache sync")
	cacheCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if !cache.WaitForCacheSync(cacheCtx.Done(), c.clients.HasSynced) {
		return fmt.Errorf("Timed out waiting for cache sync")
	}
	klog.Info("Cache synced")

	go wait.Until(
		func() { processNext(ctx, c.ingressQueue, c.sync.Ingress) },
		time.Second, ctx.Done())
	go wait.Until(
		func() { processNext(ctx, c.managedCertificateQueue, c.sync.ManagedCertificate) },
		time.Second, ctx.Done())
	go wait.Until(func() { c.synchronizeAll(ctx) }, time.Minute, ctx.Done())
	go wait.Until(func() { c.reportStatuses() }, time.Minute, ctx.Done())

	klog.Info("Waiting for stop signal or error")

	<-ctx.Done()
	klog.Info("Received stop signal, shutting down")
	return nil
}

func (c *controller) synchronizeAll(ctx context.Context) {
	if ingresses, err := c.clients.Ingress.List(); err != nil {
		runtime.HandleError(err)
	} else {
		for _, ingress := range ingresses {
			queue.Add(c.ingressQueue, ingress)
		}
	}

	if managedCertificates, err := c.clients.ManagedCertificate.List(); err != nil {
		runtime.HandleError(err)
	} else {
		for _, managedCertificate := range managedCertificates {
			queue.Add(c.managedCertificateQueue, managedCertificate)
		}
	}

	for id := range c.state.List() {
		queue.AddId(c.managedCertificateQueue, id)
	}
}

func (c *controller) reportStatuses() {
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
}

func processNext(ctx context.Context, queue workqueue.RateLimitingInterface,
	handle func(ctx context.Context, id types.Id) error) {

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
			queue.Forget(obj)
			return
		}

		queue.AddRateLimited(obj)
		runtime.HandleError(err)
	}()
}
