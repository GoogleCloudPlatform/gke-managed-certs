package controller

import (
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"managed-certs-gke/pkg/ingress"
	"time"
)

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
