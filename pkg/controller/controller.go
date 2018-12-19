/*
Copyright 2018 Google LLC

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

	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/informers/externalversions"
	mcrtlister "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/listers/gke.googleapis.com/v1alpha1"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clients"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/config"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/metrics"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/sslcertificatemanager"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/state"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/sync"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/flags"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/random"
)

type Controller struct {
	informerFactory externalversions.SharedInformerFactory
	lister          mcrtlister.ManagedCertificateLister
	metrics         metrics.Metrics
	queue           workqueue.RateLimitingInterface
	sync            sync.Sync
	synced          cache.InformerSynced
}

func New(config *config.Config, clients *clients.Clients) *Controller {
	informer := clients.InformerFactory.Gke().V1alpha1().ManagedCertificates()
	lister := informer.Lister()
	ssl := sslcertificatemanager.New(clients.Event, clients.Ssl)
	metrics := metrics.New()
	random := random.New(config.SslCertificateNamePrefix)
	sync := sync.New(clients.Clientset, lister, metrics, random, ssl, state.New(clients.ConfigMap))

	controller := &Controller{
		informerFactory: clients.InformerFactory,
		lister:          lister,
		metrics:         metrics,
		queue:           workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "queue"),
		sync:            sync,
		synced:          informer.Informer().HasSynced,
	}

	informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			controller.enqueue(obj)
		},
		UpdateFunc: func(old, new interface{}) {
			controller.enqueue(new)
		},
		DeleteFunc: func(obj interface{}) {
			controller.enqueue(obj)
		},
	})

	return controller
}

func (c *Controller) Run(ctx context.Context) error {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()

	glog.Info("Controller.Run()")

	glog.Info("Start reporting metrics")
	go c.metrics.Start(flags.F.PrometheusAddress)

	go c.informerFactory.Start(ctx.Done())

	glog.Info("Waiting for ManagedCertificate cache sync")
	if !cache.WaitForCacheSync(ctx.Done(), c.synced) {
		return fmt.Errorf("Timed out waiting for ManagedCertificate cache sync")
	}
	glog.Info("ManagedCertificate cache synced")

	go wait.Until(c.runWorker, time.Second, ctx.Done())
	go wait.Until(c.synchronizeAllMcrts, time.Minute, ctx.Done())

	glog.Info("Waiting for stop signal or error")

	<-ctx.Done()
	glog.Info("Received stop signal, shutting down")
	return nil
}

func (c *Controller) runWorker() {
	for c.processNext() {
	}
}

func (c *Controller) synchronizeAllMcrts() {
	c.sync.State()
	c.enqueueAll()
}
