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
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
)

func (c *Controller) enqueue(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	c.queue.AddRateLimited(key)
}

func (c *Controller) enqueueAll() {
	mcrts, err := c.lister.List(labels.Everything())
	if err != nil {
		runtime.HandleError(err)
		return
	}

	if len(mcrts) <= 0 {
		glog.Info("No ManagedCertificates found in cluster")
		return
	}

	var names []string
	for _, mcrt := range mcrts {
		names = append(names, mcrt.Name)
	}

	glog.Infof("Enqueuing ManagedCertificates found in cluster: %+v", names)
	for _, mcrt := range mcrts {
		c.enqueue(mcrt)
	}
}

func (c *Controller) handle(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	mcrt, err := c.lister.ManagedCertificates(namespace).Get(name)
	if err != nil {
		return err
	}

	if err := c.syncManagedCertificate(mcrt); err != nil {
		return err
	}

	_, err = c.mcrt.GkeV1alpha1().ManagedCertificates(mcrt.Namespace).Update(mcrt)
	return err
}

func (c *Controller) processNext() bool {
	obj, shutdown := c.queue.Get()

	if shutdown {
		return false
	}

	defer c.queue.Done(obj)

	key, ok := obj.(string)
	if !ok {
		c.queue.Forget(obj)
		runtime.HandleError(fmt.Errorf("Expected string in queue but got %T", obj))
		return true
	}

	err := c.handle(key)
	if err == nil {
		c.queue.Forget(obj)
		return true
	}

	c.queue.AddRateLimited(obj)
	runtime.HandleError(err)
	return true
}
