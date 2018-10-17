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
	"fmt"

	"github.com/golang/glog"
	compute "google.golang.org/api/compute/v0.beta"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"

	api "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/gke.googleapis.com/v1alpha1"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/certificate"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/http"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/random"
)

func (c *Controller) createSslCertificateIfNeeded(sslCertificateName string, mcrt *api.ManagedCertificate) (*compute.SslCertificate, error) {
	exists, err := c.ssl.Exists(sslCertificateName)
	if err != nil {
		c.event.TransientBackendError(mcrt, err)
		return nil, err
	}

	if !exists {
		// SslCertificate does not yet exist, create it
		glog.Infof("Create SslCertificate %s for Managed Certificate %s:%s, based on state", sslCertificateName, mcrt.Namespace, mcrt.Name)
		if err := c.ssl.Create(sslCertificateName, mcrt.Spec.Domains); err != nil {
			if http.IsQuotaExceeded(err) {
				c.event.TooManyCertificates(mcrt, err)
				return nil, err
			}

			c.event.TransientBackendError(mcrt, err)
			return nil, err
		}
		c.event.Create(mcrt, sslCertificateName)
	}

	sslCert, err := c.ssl.Get(sslCertificateName)
	if err != nil {
		c.event.TransientBackendError(mcrt, err)
		return nil, err
	}

	if !certificate.Equal(*mcrt, *sslCert) {
		glog.Infof("ManagedCertificate %v and SslCertificate %v are different, removing SslCertificate", mcrt, sslCert)
		if err := http.IgnoreNotFound(c.ssl.Delete(sslCertificateName)); err != nil {
			c.event.TransientBackendError(mcrt, err)
			return nil, err
		}

		c.event.Delete(mcrt, sslCertificateName)
		glog.Infof("SslCertificate %s deleted", sslCertificateName)
	}

	return sslCert, nil
}

func (c *Controller) createSslCertificateNameIfNeeded(mcrt *api.ManagedCertificate) (string, error) {
	sslCertificateName, exists := c.state.Get(mcrt.Namespace, mcrt.Name)

	if exists && sslCertificateName != "" {
		return sslCertificateName, nil
	}

	// State does not have anything for this Managed Certificate or no SslCertificate is associated with it
	sslCertificateName, err := c.randomName()
	if err != nil {
		return "", err
	}

	glog.Infof("Add to state SslCertificate name %s for Managed Certificate %s:%s", sslCertificateName, mcrt.Namespace, mcrt.Name)
	c.state.Put(mcrt.Namespace, mcrt.Name, sslCertificateName)
	return sslCertificateName, nil
}

func (c *Controller) handle(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	mcrt, err := c.lister.ManagedCertificates(namespace).Get(name)
	if err != nil {
		return err
	}

	glog.Infof("Handling Managed Certificate %s:%s", mcrt.Namespace, mcrt.Name)

	sslCertificateName, err := c.createSslCertificateNameIfNeeded(mcrt)
	if err != nil {
		return err
	}

	sslCert, err := c.createSslCertificateIfNeeded(sslCertificateName, mcrt)
	if err != nil {
		return err
	}

	if err := certificate.CopyStatus(*sslCert, mcrt); err != nil {
		return err
	}

	_, err = c.mcrt.GkeV1alpha1().ManagedCertificates(mcrt.Namespace).Update(mcrt)
	return err
}

func (c *Controller) processNext() bool {
	obj, shutdown := c.queue.Get()

	if shutdown {
		return false
	}

	defer c.queue.Done(obj)

	key, ok := obj.(string)
	if !ok {
		c.queue.Forget(obj)
		runtime.HandleError(fmt.Errorf("Expected string in queue but got %T", obj))
		return true
	}

	err := c.handle(key)
	if err == nil {
		c.queue.Forget(obj)
		return true
	}

	c.queue.AddRateLimited(obj)
	runtime.HandleError(err)
	return true
}

func (c *Controller) runWorker() {
	for c.processNext() {
	}
}

func (c *Controller) randomName() (string, error) {
	name, err := random.Name()
	if err != nil {
		return "", err
	}

	exists, err := c.ssl.Exists(name)
	if err != nil {
		return "", err
	}

	if exists {
		// Name taken, choose a new one
		name, err = random.Name()
		if err != nil {
			return "", err
		}
	}

	return name, nil
}

// Deletes SslCertificate mapped to already deleted Managed Certificate.
func (c *Controller) removeSslCertificate(namespace, name string) error {
	sslCertificateName, exists := c.state.Get(namespace, name)
	if !exists {
		glog.Infof("Can't find in state SslCertificate for Managed Certificate %s:%s", namespace, name)
		return nil
	}

	// SslCertificate (still) exists in state, check if it exists in GCE
	exists, err := c.ssl.Exists(sslCertificateName)
	if err != nil {
		return err
	}

	if !exists {
		// SslCertificate already deleted
		glog.Infof("SslCertificate %s for Managed Certificate %s:%s already deleted", sslCertificateName, namespace, name)
		return nil
	}

	// SslCertificate exists in GCE, remove it and delete entry from state
	if err := http.IgnoreNotFound(c.ssl.Delete(sslCertificateName)); err != nil {
		return err
	}

	// Successfully deleted SslCertificate
	glog.Infof("Successfully deleted SslCertificate %s for Managed Certificate %s:%s", sslCertificateName, namespace, name)
	return nil
}

// Handles clean up. If a Managed Certificate gets deleted, it's noticed here that it still exists in controller's state,
// the mapped SslCertificate is deleted and state entry is removed.
func (c *Controller) synchronizeState() {
	for _, key := range c.state.GetAllKeys() {
		// key identifies a Managed Certificate known in state
		if _, err := c.lister.ManagedCertificates(key.Namespace).Get(key.Name); http.IsNotFound(err) {
			// Managed Certificate exists in state, but does not exist in cluster
			glog.Infof("Managed Certificate %s:%s in state but not in cluster", key.Namespace, key.Name)

			err := c.removeSslCertificate(key.Namespace, key.Name)
			if err == nil {
				// SslCertificate mapped to Managed Certificate is already deleted. Remove entry from state.
				c.state.Delete(key.Namespace, key.Name)
			} else {
				runtime.HandleError(err)
			}
		}
	}
}

func (c *Controller) synchronizeAllMcrts() {
	c.synchronizeState()
	c.enqueueAll()
}
