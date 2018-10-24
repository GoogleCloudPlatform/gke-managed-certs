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
	"fmt"
	"time"

	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/client"
	mcrtlister "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/listers/gke.googleapis.com/v1alpha1"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/sslcertificatemanager"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/state"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/sync"
)

type Controller struct {
	lister mcrtlister.ManagedCertificateLister
	queue  workqueue.RateLimitingInterface
	sync   sync.Sync
	synced cache.InformerSynced
}

func New(clients *client.Clients) *Controller {
	informer := clients.InformerFactory.Gke().V1alpha1().ManagedCertificates()
	lister := informer.Lister()

	controller := &Controller{
		lister: lister,
		queue:  workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "queue"),
		sync:   sync.New(clients.Clientset, lister, sslcertificatemanager.New(clients.Event, clients.Ssl), state.New(clients.ConfigMap)),
		synced: informer.Informer().HasSynced,
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

func (c *Controller) Run(stopChannel <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()

	glog.Info("Controller.Run()")

	glog.Info("Waiting for Managed Certificate cache sync")
	if !cache.WaitForCacheSync(stopChannel, c.synced) {
		return fmt.Errorf("Timed out waiting for Managed Certificate cache sync")
	}
	glog.Info("Managed Certificate cache synced")

	go wait.Until(c.runWorker, time.Second, stopChannel)
	go wait.Until(c.synchronizeAllMcrts, time.Minute, stopChannel)

	glog.Info("Controller waiting for stop signal or error")

	<-stopChannel
	glog.Info("Controller received stop signal")

	glog.Info("Controller shutting down")
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
