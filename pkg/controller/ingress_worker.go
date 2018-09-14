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
	api "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"

	"managed-certs-gke/pkg/utils/annotation"
)

func (c *IngressController) runWatcher() {
	watcher, err := c.ingress.Watch()

	if err != nil {
		runtime.HandleError(err)
		return
	}

	for {
		select {
		case event := <-watcher.ResultChan():
			if event.Object != nil {
				if ingress, ok := event.Object.(*api.Ingress); !ok {
					runtime.HandleError(fmt.Errorf("Expected an Ingress, watch returned %T instead, event: %+v", event.Object, event))
				} else if event.Type == watch.Added || event.Type == watch.Modified {
					c.enqueue(ingress)
				}
			}
		default:
		}

		if c.queue.ShuttingDown() {
			watcher.Stop()
			return
		}

		time.Sleep(c.ingressWatcherDelay)
	}
}

func (c *Controller) runIngressWorker() {
	for c.processNextIngress() {
	}
}

func (c *Controller) handleIngress(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	glog.Infof("Handling ingress %s:%s", namespace, name)

	ingress, err := c.Ingress.ingress.Get(namespace, name)
	if err != nil {
		return err
	}

	mcrtNames, isNonEmpty := annotation.Parse(ingress)
	if !isNonEmpty {
		// There is either no annotation on this ingress, or there is one which has an empty value
		return nil
	}

	for _, name := range mcrtNames {
		// Assume the namespace is the same as ingress's
		glog.Infof("Looking up Managed Certificate %s:%s", namespace, name)
		if mcrt, ok := c.Mcrt.getMcrt(namespace, name); ok {
			glog.Infof("Enqueue Managed Certificate %s:%s for further processing", namespace, name)
			c.Mcrt.enqueue(mcrt)
		}
	}

	return nil
}

func (c *Controller) processNextIngress() bool {
	obj, shutdown := c.Ingress.queue.Get()

	if shutdown {
		return false
	}

	defer c.Ingress.queue.Done(obj)

	key, ok := obj.(string)
	if !ok {
		c.Ingress.queue.Forget(obj)
		runtime.HandleError(fmt.Errorf("Expected string in queue but got %T", obj))
		return true
	}

	err := c.handleIngress(key)
	if err == nil {
		c.Ingress.queue.Forget(obj)
		return true
	}

	c.Ingress.queue.AddRateLimited(obj)
	runtime.HandleError(err)
	return true
}
