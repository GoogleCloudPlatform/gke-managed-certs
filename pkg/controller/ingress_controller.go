package controller

import (
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"time"
)

func (c *IngressController) Run(stopChannel <-chan struct{}) {
	defer c.queue.ShutDown()

	go c.runWatcher()
	go wait.Until(c.enqueueAll, 1*time.Minute, stopChannel)

	<-stopChannel
}

func (c *IngressController) enqueue(obj interface{}) {
	if key, err := cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
	} else {
		c.queue.AddRateLimited(key)
	}
}

func (c *IngressController) enqueueAll() {
	ingresses, err := c.client.List()
	if err != nil {
		runtime.HandleError(err)
		return
	}

	for _, ing := range ingresses.Items {
		c.enqueue(&ing)
	}
}
