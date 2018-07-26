package controller

import (
	"fmt"
	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	//mcertlister "managed-certs-gke/pkg/client/listers/cloud.google.com/v1alpha1"
)

func (c *McertController) runWorker() {
	for c.processNext() {
	}
}

func (c *McertController) processNext() bool {
	obj, shutdown := c.queue.Get()

	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer c.queue.Done(obj)

		if key, ok := obj.(string); !ok {
			c.queue.Forget(obj)
			return fmt.Errorf("Expected string in mcertQueue but got %#v", obj)
		} else {
			ns, name, err := cache.SplitMetaNamespaceKey(key)
			if err != nil {
				return err
			}
			glog.Infof("Handling ManagedCertificate %s.%s", ns, name)

			_, err = c.lister.ManagedCertificates(ns).Get(name)
			if err != nil {
				return err
			}
		}

		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
	}

	return true
}
