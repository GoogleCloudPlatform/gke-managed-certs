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
	"github.com/golang/glog"
	api "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"strings"
	"time"
)

const (
	annotation = "cloud.google.com/managed-certificates"
	splitBy = ","
)


func (c *IngressController) runWatcher() {
	watcher, err := c.client.Watch()

	if err != nil {
		runtime.HandleError(err)
		return
	}

	for {
		select {
		case event := <-watcher.ResultChan():
			if event.Object != nil {
				if ing, ok := event.Object.(*api.Ingress); !ok {
					runtime.HandleError(fmt.Errorf("Expected an Ingress, watch returned %v instead, event: %v", event.Object, event))
				} else if event.Type == watch.Added || event.Type == watch.Modified {
					c.enqueue(ing)
				}
			}
		default:
		}

		if c.queue.ShuttingDown() {
			watcher.Stop()
			return
		}

		time.Sleep(time.Second)
	}
}

func (c *Controller) runIngressWorker() {
	for c.processNextIngress() {
	}
}

func parseAnnotation(annotationValue string) (result []string) {
	if annotationValue == "" {
		return
	}

	for _, value := range strings.Split(annotationValue, splitBy) {
		result = append(result, strings.TrimSpace(value))
	}

	return
}

func (c *Controller) handleIngress(key string) error {
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	glog.Infof("Handling ingress %s.%s", ns, name)

	ing, err := c.Ingress.client.Get(ns, name)
	if err != nil {
		return err
	}

	annotationValue, present := ing.ObjectMeta.Annotations[annotation]
	if !present {
		// There is no annotation on this ingress
		return nil
	}

	glog.Infof("Found annotation %s", annotationValue)

	for _, name := range parseAnnotation(annotationValue) {
		// Assume the namespace is the same as ingress's
		glog.Infof("Looking up managed certificate %s in namespace %s", name, ns)
		mcert, err := c.Mcert.lister.ManagedCertificates(ns).Get(name)

		if err != nil {
			// TODO generate k8s event - can't fetch mcert
			runtime.HandleError(err)
		} else {
			glog.Infof("Enqueue managed certificate %s for further processing", name)
			c.Mcert.enqueue(mcert)
		}
	}

	return nil
}

func (c *Controller) processNextIngress() bool {
	obj, shutdown := c.Ingress.queue.Get()

	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer c.Ingress.queue.Done(obj)

		var key string
		var ok bool
		if key, ok = obj.(string); !ok {
			c.Ingress.queue.Forget(obj)
			return fmt.Errorf("Expected string in ingressQueue but got %#v", obj)
		}

		if err := c.handleIngress(key); err != nil {
			c.Ingress.queue.AddRateLimited(obj)
			return err
		}

		c.Ingress.queue.Forget(obj)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
	}

	return true
}
