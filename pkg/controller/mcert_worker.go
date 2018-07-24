package controller

import (
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	//mcertlister "managed-certs-gke/pkg/client/listers/cloud.google.com/v1alpha1"
)

func (c *Controller) enqueueMcert(obj interface{}) {
	if key, err := cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
	} else {
		c.mcertQueue.AddRateLimited(key)
	}
}

func (c *Controller) runMcertWorker() {
	for c.processNextMcert() {
	}
}

func (c *Controller) processNextMcert() bool {
	return true
}

func (c *Controller) enqueueAllMcerts() {
	mcerts, err := c.mcertLister.List(labels.Everything())
	if err != nil {
		runtime.HandleError(err)
		return
	}

	for _, mcert := range mcerts {
		c.enqueueMcert(mcert)
	}
}
