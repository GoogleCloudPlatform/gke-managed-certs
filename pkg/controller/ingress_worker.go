package controller

import (
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"managed-certs-gke/pkg/ingress"
)

const (
	annotation = "cloud.google.com/managed-certificates"
)

func (c *Controller) runIngressWorker() {
	for c.processNextIngress() {
	}
}

func parseAnnotation(annotationValue string) (names []string, err error) {
	err = json.Unmarshal([]byte(annotationValue), &names)
	return
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
				mcert, err := c.mcertLister.ManagedCertificates(ns).Get(name)

				if err != nil {
					// TODO generate k8s event - can't fetch mcert
					runtime.HandleError(err)
				} else {
					glog.Infof("Enqueue managed certificate %s for further processing", name)
					c.enqueueMcert(mcert)
				}
			}
		}

		c.ingressQueue.Forget(obj)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
	}

	return true
}
