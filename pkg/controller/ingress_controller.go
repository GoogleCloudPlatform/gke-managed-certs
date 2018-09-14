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
	"time"

	api "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"managed-certs-gke/pkg/client/ingress"
)

type IngressController struct {
	ingress             *ingress.Ingress
	queue               workqueue.RateLimitingInterface
	ingressWatcherDelay time.Duration
}

func (c *IngressController) Run(stopChannel <-chan struct{}) {
	defer c.queue.ShutDown()

	go c.runWatcher()
	go wait.Until(c.synchronizeAllIngresses, time.Minute, stopChannel)

	<-stopChannel
}

func (c *IngressController) enqueue(ingress *api.Ingress) {
	key, err := cache.MetaNamespaceKeyFunc(ingress)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	c.queue.AddRateLimited(key)
}

func (c *IngressController) synchronizeAllIngresses() {
	ingresses, err := c.ingress.List()
	if err != nil {
		runtime.HandleError(err)
		return
	}

	for _, ingress := range ingresses.Items {
		c.enqueue(&ingress)
	}
}
