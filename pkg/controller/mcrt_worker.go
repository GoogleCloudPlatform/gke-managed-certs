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
	compute "google.golang.org/api/compute/v0.alpha"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"

	api "managed-certs-gke/pkg/apis/gke.googleapis.com/v1alpha1"
	"managed-certs-gke/pkg/utils/equal"
	"managed-certs-gke/pkg/utils/random"
)

const (
	sslActive                              = "ACTIVE"
	sslFailedNotVisible                    = "FAILED_NOT_VISIBLE"
	sslFailedCaaChecking                   = "FAILED_CAA_CHECKING"
	sslFailedCaaForbidden                  = "FAILED_CAA_FORBIDDEN"
	sslFailedRateLimited                   = "FAILED_RATE_LIMITED"
	sslManagedCertificateStatusUnspecified = "MANAGED_CERTIFICATE_STATUS_UNSPECIFIED"
	sslProvisioning                        = "PROVISIONING"
	sslProvisioningFailed                  = "PROVISIONING_FAILED"
	sslProvisioningFailedPermanently       = "PROVISIONING_FAILED_PERMANENTLY"
	sslRenewalFailed                       = "RENEWAL_FAILED"
)

func translateDomainStatus(status string) (string, error) {
	switch status {
	case sslProvisioning:
		return "Provisioning", nil
	case sslFailedNotVisible:
		return "FailedNotVisible", nil
	case sslFailedCaaChecking:
		return "FailedCaaChecking", nil
	case sslFailedCaaForbidden:
		return "FailedCaaForbidden", nil
	case sslFailedRateLimited:
		return "FailedRateLimited", nil
	case sslActive:
		return "Active", nil
	default:
		return "", fmt.Errorf("Unexpected status %s", status)
	}
}

func translateSslCertStatus(sslCert *compute.SslCertificate) (string, error) {
	switch sslCert.Managed.Status {
	case sslActive:
		return "Active", nil
	case sslManagedCertificateStatusUnspecified, "":
		return "", nil
	case sslProvisioning:
		return "Provisioning", nil
	case sslProvisioningFailed:
		return "ProvisioningFailed", nil
	case sslProvisioningFailedPermanently:
		return "ProvisioningFailedPermanently", nil
	case sslRenewalFailed:
		return "RenewalFailed", nil
	default:
		return "", fmt.Errorf("Unexpected status %s of SslCertificate %v", sslCert.Managed.Status, sslCert)
	}
}

func (c *McrtController) updateStatus(mcrt *api.ManagedCertificate) error {
	sslCertificateName, exists := c.state.Get(mcrt.Namespace, mcrt.Name)
	if !exists {
		return fmt.Errorf("Failed to find in state Managed Certificate %s", mcrt.Name)
	}

	sslCert, err := c.ssl.Get(sslCertificateName)
	if err != nil {
		return err
	}

	mcrtStatus, err := translateSslCertStatus(sslCert)
	if err != nil {
		return err
	}
	mcrt.Status.CertificateStatus = mcrtStatus

	domainStatus := make([]api.DomainStatus, 0)
	for domain, status := range sslCert.Managed.DomainStatus {
		translatedStatus, err := translateDomainStatus(status)
		if err != nil {
			return err
		}

		domainStatus = append(domainStatus, api.DomainStatus{
			Domain: domain,
			Status: translatedStatus,
		})
	}
	mcrt.Status.DomainStatus = domainStatus

	mcrt.Status.CertificateName = sslCert.Name

	_, err = c.mcrt.GkeV1alpha1().ManagedCertificates(mcrt.Namespace).Update(mcrt)
	return err
}

func (c *McrtController) createSslCertificateIfNeeded(sslCertificateName string, mcrt *api.ManagedCertificate) error {
	if !c.ssl.Exists(sslCertificateName) {
		//SslCertificate does not yet exist, create it
		glog.Infof("McrtController: create a new SslCertificate %s associated with Managed Certificate %s, based on state", sslCertificateName, mcrt.Name)
		if err := c.ssl.Create(sslCertificateName, mcrt.Spec.Domains); err != nil {
			return err
		}
	}

	sslCert, err := c.ssl.Get(sslCertificateName)
	if err != nil {
		return err
	}

	if !equal.Certificates(*mcrt, *sslCert) {
		glog.Infof("McrtController: ManagedCertificate %v and SslCertificate %v are different, removing the SslCertificate", mcrt, sslCert)
		err := c.ssl.Delete(sslCertificateName)
		if err != nil {
			return err
		}

		glog.Infof("McrtController: SslCertificate %s deleted", sslCertificateName)
	}

	return nil
}

func (c *McrtController) createSslCertificateNameIfNeeded(mcrt *api.ManagedCertificate) (string, error) {
	sslCertificateName, exists := c.state.Get(mcrt.Namespace, mcrt.Name)

	if exists && sslCertificateName != "" {
		return sslCertificateName, nil
	}

	//State does not have anything for this Managed Certificate or no SslCertificate is associated with it
	sslCertificateName, err := c.randomName()
	if err != nil {
		return "", err
	}

	glog.Infof("McrtController: add to state SslCertificate name %s associated with Managed Certificate %s:%s", sslCertificateName, mcrt.Namespace, mcrt.Name)
	c.state.Put(mcrt.Namespace, mcrt.Name, sslCertificateName)
	return sslCertificateName, nil
}

func (c *McrtController) handleMcrt(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	mcrt, ok := c.getMcrt(namespace, name)
	if !ok {
		return nil
	}

	glog.Infof("McrtController: handling Managed Certificate %s:%s", mcrt.Namespace, mcrt.Name)

	sslCertificateName, err := c.createSslCertificateNameIfNeeded(mcrt)
	if err != nil {
		return err
	}

	if err = c.createSslCertificateIfNeeded(sslCertificateName, mcrt); err != nil {
		return err
	}

	return c.updateStatus(mcrt)
}

func (c *McrtController) processNext() bool {
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

	err := c.handleMcrt(key)
	if err == nil {
		c.queue.Forget(obj)
		return true
	}

	c.queue.AddRateLimited(obj)
	runtime.HandleError(err)
	return true
}

func (c *McrtController) runWorker() {
	for c.processNext() {
	}
}

func (c *McrtController) randomName() (string, error) {
	name, err := random.Name()
	if err != nil {
		return "", err
	}

	if c.ssl.Exists(name) {
		// Name taken, choose a new one
		name, err = random.Name()
		if err != nil {
			return "", err
		}
	}

	return name, nil
}
