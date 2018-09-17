/*
Copyright 2018 Google LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"time"

	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	api "managed-certs-gke/pkg/apis/gke.googleapis.com/v1alpha1"
	"managed-certs-gke/pkg/client/ssl"
	"managed-certs-gke/third_party/client/clientset/versioned"
	mcrtlister "managed-certs-gke/third_party/client/listers/gke.googleapis.com/v1alpha1"
)

type McrtController struct {
	mcrt   *versioned.Clientset
	lister mcrtlister.ManagedCertificateLister
	synced cache.InformerSynced
	queue  workqueue.RateLimitingInterface
	ssl    *ssl.SSL
	state  *McrtState
}

func (c *McrtController) Run(stopChannel <-chan struct{}) {
	defer c.queue.ShutDown()

	go wait.Until(c.runWorker, time.Second, stopChannel)
	go wait.Until(c.synchronizeAllMcrts, time.Minute, stopChannel)

	<-stopChannel
}

func (c *McrtController) enqueue(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	c.queue.AddRateLimited(key)
}

func (c *McrtController) getMcrt(namespace, name string) (*api.ManagedCertificate, bool) {
	mcrt, err := c.lister.ManagedCertificates(namespace).Get(name)
	if err == nil {
		return mcrt, true
	}

	//TODO(krzyk) generate k8s event - can't fetch mcrt
	runtime.HandleError(err)
	return nil, false
}

// Deletes SslCertificate mapped to already deleted Managed Certificate.
func (c *McrtController) removeSslCertificate(namespace, name string) error {
	sslCertificateName, exists := c.state.Get(namespace, name)
	if !exists {
		glog.Infof("McrtController: Can't find in state SslCertificate mapped to Managed Certificate %s:%s", namespace, name)
		return nil
	}

	// SslCertificate (still) exists in state, check if it exists in GCE
	exists, err := c.ssl.Exists(sslCertificateName)
	if err != nil {
		return err
	}

	if !exists {
		// SslCertificate already deleted
		glog.Infof("McrtController: SslCertificate %s mapped to Managed Certificate %s:%s already deleted", sslCertificateName, namespace, name)
		return nil
	}

	// SslCertificate exists in GCE, remove it and delete entry from state
	if err := c.ssl.Delete(sslCertificateName); err != nil {
		// Failed to delete SslCertificate
		// TODO(krzyk): generate k8s event
		runtime.HandleError(err)
		return err
	}

	// Successfully deleted SslCertificate
	glog.Infof("McrtController: successfully deleted SslCertificate %s mapped to Managed Certificate %s:%s", sslCertificateName, namespace, name)
	return nil
}

/*
* Handles clean up. If a Managed Certificate gets deleted, it's noticed here and acted upon.
 */
func (c *McrtController) synchronizeState() {
	for _, key := range c.state.GetAllKeys() {
		// key identifies a Managed Certificate known in state
		if _, ok := c.getMcrt(key.Namespace, key.Name); !ok {
			// Managed Certificate exists in state, but does not exist in cluster
			glog.Infof("McrtController: Managed Certificate %s:%s in state but not in cluster", key.Namespace, key.Name)

			if c.removeSslCertificate(key.Namespace, key.Name) == nil {
				// SslCertificate mapped to Managed Certificate is already deleted. Remove entry from state.
				c.state.Delete(key.Namespace, key.Name)
			}
		}
	}
}

func (c *McrtController) enqueueAll() {
	mcrts, err := c.lister.List(labels.Everything())
	if err != nil {
		//TODO(krzyk) generate k8s event - can't fetch mcrt
		runtime.HandleError(err)
		return
	}

	if len(mcrts) <= 0 {
		glog.Info("McrtController: no Managed Certificates found in cluster")
		return
	}

	var names []string
	for _, mcrt := range mcrts {
		names = append(names, mcrt.Name)
	}

	glog.Infof("McrtController: enqueuing Managed Certificates found in cluster: %+v", names)
	for _, mcrt := range mcrts {
		c.enqueue(mcrt)
	}
}

func (c *McrtController) synchronizeAllMcrts() {
	c.synchronizeState()
	c.enqueueAll()
}
