package controller

import (
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"time"
)

const (
	annotation = "cloud.google.com/managed-certificates"
)


func (c *IngressController) runWatcher() {
	watcher, err := c.client.Watch()

	if err != nil {
		runtime.HandleError(err)
		return
	}

	for {
		select {
		case event := <-watcher.ResultChan():
			if event.Type == watch.Added || event.Type == watch.Modified {
				c.enqueue(event.Object)
			}
		default:
		}

		if c.queue.ShuttingDown() {
			watcher.Stop()
			return
		}

		time.Sleep(time.Second)
	}
}

func (c *Controller) runIngressWorker() {
	for c.processNextIngress() {
	}
}

func parseAnnotation(annotationValue string) (names []string, err error) {
	err = json.Unmarshal([]byte(annotationValue), &names)
	return
}

func (c *Controller) processNextIngress() bool {
	obj, shutdown := c.Ingress.queue.Get()

	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer c.Ingress.queue.Done(obj)

		if key, ok := obj.(string); !ok {
			c.Ingress.queue.Forget(obj)
			return fmt.Errorf("Expected string in ingressQueue but got %#v", obj)
		} else {
			ns, name, err := cache.SplitMetaNamespaceKey(key)
			if err != nil {
				return err
			}
			glog.Infof("Handling ingress %s.%s", ns, name)

			ing, err := c.Ingress.client.Get(ns, name)
			if err != nil {
				return err
			}

			annotationValue, present := ing.ObjectMeta.Annotations[annotation]
			if !present {
				// There is no annotation on this ingress
				return nil
			}

			glog.Infof("Found annotation %s", annotationValue)

			names, err := parseAnnotation(annotationValue)
			if err != nil {
				// Unable to parse annotations
				return err
			}

			for _, name := range names {
				// Assume the namespace is the same as ingress's
				glog.Infof("Looking up managed certificate %s in namespace %s", name, ns)
				mcert, err := c.Mcert.lister.ManagedCertificates(ns).Get(name)

				if err != nil {
					// TODO generate k8s event - can't fetch mcert
					runtime.HandleError(err)
				} else {
					glog.Infof("Enqueue managed certificate %s for further processing", name)
					c.Mcert.enqueue(mcert)
				}
			}
		}

		c.Ingress.queue.Forget(obj)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
	}

	return true
}
