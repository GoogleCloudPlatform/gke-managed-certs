package controller

import (
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	//mcertlister "managed-certs-gke/pkg/client/listers/cloud.google.com/v1alpha1"
)

func (c *McertController) enqueue(obj interface{}) {
	if key, err := cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
	} else {
		c.queue.AddRateLimited(key)
	}
}

func (c *McertController) runWorker() {
	for c.processNext() {
	}
}

func (c *McertController) processNext() bool {
	return true
}

func (c *McertController) enqueueAll() {
	mcerts, err := c.lister.List(labels.Everything())
	if err != nil {
		runtime.HandleError(err)
		return
	}

	for _, mcert := range mcerts {
		c.enqueue(mcert)
	}
}
