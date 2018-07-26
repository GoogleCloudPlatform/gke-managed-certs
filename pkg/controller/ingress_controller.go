package controller

import (
	"k8s.io/apimachinery/pkg/util/wait"
	"time"
)

func (c *IngressController) Run(stopChannel <-chan struct{}) {
	defer c.queue.ShutDown()

	go c.runWatcher()
	go wait.Until(c.enqueueAll, 15*time.Minute, stopChannel)

	<-stopChannel
}
