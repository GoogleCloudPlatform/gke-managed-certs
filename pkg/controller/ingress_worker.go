package controller

import (
	"fmt"
	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"managed-certs-gke/pkg/ingress"
)

func (c *Controller) runIngressWorker() {
	for c.processNextIngress() {
	}
}

func (c *Controller) processNextIngress() bool {
	obj, shutdown := c.ingressQueue.Get()

	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer c.ingressQueue.Done(obj)

		if key, ok := obj.(string); !ok {
			c.ingressQueue.Forget(obj)
			return fmt.Errorf("Expected string in ingressQueue but got %#v", obj)
		} else {
			ns, name, err := cache.SplitMetaNamespaceKey(key)
			if err != nil {
				return err
			}
			glog.Infof("Handling ingress %s.%s", ns, name)

			ing, err := ingress.Get(c.ingressClient, ns, name)
			if err != nil {
				return err
			}

			glog.Infof("%v", ing)

			//TODO: read annotation and add mcert to queue
		}

		c.ingressQueue.Forget(obj)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
	}

	return true
}
