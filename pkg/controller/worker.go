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
	"github.com/golang/glog"
	compute "google.golang.org/api/compute/v0.beta"
	"k8s.io/apimachinery/pkg/util/runtime"

	api "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/gke.googleapis.com/v1alpha1"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/certificates"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/http"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/random"
)

func (c *Controller) createSslCertificateIfNeeded(sslCertificateName string, mcrt *api.ManagedCertificate) (*compute.SslCertificate, error) {
	exists, err := c.ssl.Exists(sslCertificateName, mcrt)
	if err != nil {
		return nil, err
	}

	if !exists {
		// SslCertificate does not exist, create it
		if err := c.ssl.Create(sslCertificateName, *mcrt); err != nil {
			return nil, err
		}
	}

	sslCert, err := c.ssl.Get(sslCertificateName, mcrt)
	if err != nil {
		return nil, err
	}

	if !certificates.Equal(*mcrt, *sslCert) {
		glog.Infof("ManagedCertificate %v and SslCertificate %v are different, deleting SslCertificate", mcrt, sslCert)
		if err := c.ssl.Delete(sslCertificateName, mcrt); err != nil {
			return nil, err
		}
	}

	return sslCert, nil
}

func (c *Controller) createSslCertificateNameIfNeeded(mcrt *api.ManagedCertificate) (string, error) {
	sslCertificateName, exists := c.state.Get(mcrt.Namespace, mcrt.Name)

	if exists && sslCertificateName != "" {
		return sslCertificateName, nil
	}

	// Generate a new name for SslCertificate associated with this ManagedCertificate and put it in state
	sslCertificateName, err := random.Name()
	if err != nil {
		return "", err
	}

	glog.Infof("Add to state SslCertificate name %s for ManagedCertificate %s:%s", sslCertificateName, mcrt.Namespace, mcrt.Name)
	c.state.Put(mcrt.Namespace, mcrt.Name, sslCertificateName)
	return sslCertificateName, nil
}

func (c *Controller) syncManagedCertificate(mcrt *api.ManagedCertificate) error {
	glog.Infof("Syncing ManagedCertificate %s:%s", mcrt.Namespace, mcrt.Name)

	sslCertificateName, err := c.createSslCertificateNameIfNeeded(mcrt)
	if err != nil {
		return err
	}

	sslCert, err := c.createSslCertificateIfNeeded(sslCertificateName, mcrt)
	if err != nil {
		return err
	}

	if err := certificates.CopyStatus(*sslCert, mcrt); err != nil {
		return err
	}

	return nil
}

// Deletes SslCertificate mapped to already deleted ManagedCertificate.
func (c *Controller) deleteSslCertificateIfOrphaned(namespace, name, sslCertificateName string) error {
	exists, err := c.ssl.Exists(sslCertificateName, nil)
	if err != nil {
		return err
	}

	if !exists {
		glog.Infof("SslCertificate %s for ManagedCertificate %s:%s already deleted", sslCertificateName, namespace, name)
		return nil
	}

	glog.Infof("Delete SslCertificate %s, because ManagedCertificate %s:%s already deleted", sslCertificateName, namespace, name)
	if err := c.ssl.Delete(sslCertificateName, nil); err != nil {
		return err
	}

	return nil
}

// Handles clean up. If a Managed Certificate gets deleted, it's noticed here that it still exists in controller's state,
// the mapped SslCertificate is deleted and state entry is removed.
func (c *Controller) syncState() {
	c.state.Foreach(func(namespace, name, sslCertificateName string) {
		if _, err := c.lister.ManagedCertificates(namespace).Get(name); http.IsNotFound(err) {
			glog.Infof("ManagedCertificate %s:%s in state, but has been deleted from cluster", namespace, name)

			err := c.deleteSslCertificateIfOrphaned(namespace, name, sslCertificateName)
			if err != nil {
				runtime.HandleError(err)
				return
			}

			c.state.Delete(namespace, name)
		}
	})
}

func (c *Controller) synchronizeAllMcrts() {
	c.syncState()
	c.enqueueAll()
}
