package controller

import (
	"fmt"
	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"time"
)

func (c *McertController) Run(stopChannel <-chan struct{}, errors chan<- error) {
	defer c.queue.ShutDown()

	glog.Info("Waiting for managedcertificate cache sync")
	if !cache.WaitForCacheSync(stopChannel, c.synced) {
		errors <- fmt.Errorf("Timed out waiting for cache sync")
		return
	}
	glog.Info("Cache synced")

	err := c.initializeState()
	if err != nil {
		errors <- fmt.Errorf("Cnuld not intialize state: %v", err)
		return
	}

	go wait.Until(c.runWorker, time.Second, stopChannel)
	go wait.Until(c.enqueueAll, 15*time.Minute, stopChannel)

	<-stopChannel
}

func (c *McertController) initializeState() error {
	mcerts, err := c.lister.List(labels.Everything())
	if err != nil {
		return err
	}

	c.state.Lock()
	for _, mcert := range mcerts {
		c.state.m[mcert.ObjectMeta.Name] = mcert.Status.CertificateName
	}
	c.state.Unlock()

	return nil
}
