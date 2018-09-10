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

	"managed-certs-gke/pkg/client"
)

type Controller struct {
	Ingress IngressController
	Mcrt    McrtController
}

func NewController(clients *client.Clients) *Controller {
	mcrtInformer := clients.McrtInformerFactory.Gke().V1alpha1().ManagedCertificates()

	controller := &Controller{
		Ingress: IngressController{
			ingress: clients.Ingress,
			queue:   workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "ingressQueue"),
		},
		Mcrt: McrtController{
			mcrt:   clients.Mcrt,
			lister: mcrtInformer.Lister(),
			synced: mcrtInformer.Informer().HasSynced,
			queue:  workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "mcrtQueue"),
			ssl:    clients.Ssl,
			state:  newMcrtState(),
		},
	}

	mcrtInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			controller.Mcrt.enqueue(obj)
		},
		UpdateFunc: func(old, new interface{}) {
			controller.Mcrt.enqueue(new)
		},
		DeleteFunc: func(obj interface{}) {
			controller.Mcrt.enqueue(obj)
		},
	})

	return controller
}

func (c *Controller) Run(stopChannel <-chan struct{}, ingressWatcherDelay time.Duration) error {
	defer runtime.HandleCrash()

	done := make(chan struct{})
	defer close(done)

	glog.Info("Controller.Run()")

	glog.Info("Waiting for Managed Certificate cache sync")
	if !cache.WaitForCacheSync(stopChannel, c.Mcrt.synced) {
		return fmt.Errorf("Timed out waiting for Managed Certificate cache sync")
	}
	glog.Info("Managed Certificate cache synced")

	errors := make(chan error)
	go c.Mcrt.Run(done, errors)
	go c.Ingress.Run(done, ingressWatcherDelay)

	go wait.Until(c.runIngressWorker, time.Second, stopChannel)
	go wait.Until(c.updatePreSharedCertAnnotation, time.Minute, stopChannel)

	glog.Info("Controller waiting for stop signal or error")
	select {
	case <-stopChannel:
		glog.Info("Controller received stop signal")
	case err := <-errors:
		runtime.HandleError(err)
	}

	glog.Info("Controller shutting down")
	return nil
}
