package controller

import (
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"managed-certs-gke/pkg/ingress"
	"time"
)

func (c *Controller) enqueueIngress(obj interface{}) {
	if key, err := cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
	} else {
		c.ingressQueue.AddRateLimited(key)
	}
}

func (c *Controller) runIngressWatcher() {
	ingressWatcher, err := ingress.Watch(c.ingressClient)

	if err != nil {
		runtime.HandleError(err)
		return
	}

	for {
		select {
		case event := <-ingressWatcher.ResultChan():
			if event.Type == watch.Added || event.Type == watch.Modified {
				c.enqueueIngress(event.Object)
			}
		default:
		}

		if c.ingressQueue.ShuttingDown() {
			ingressWatcher.Stop()
			return
		}

		time.Sleep(time.Second)
	}
}

func (c *Controller) enqueueAllIngresses() {
	ingresses, err := ingress.List(c.ingressClient)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	for _, ing := range ingresses.Items {
		c.enqueueIngress(ing)
	}
}
