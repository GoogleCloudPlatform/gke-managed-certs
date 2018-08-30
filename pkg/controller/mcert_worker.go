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
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"

	"managed-certs-gke/pkg/utils"

	api "managed-certs-gke/pkg/apis/alpha.cloud.google.com/v1alpha1"
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
		return "", fmt.Errorf("Unexpected status %v", status)
	}
}

func (c *McertController) updateStatus(mcert *api.ManagedCertificate) error {
	sslCertificateState, exists := c.state.Get(mcert.ObjectMeta.Name)
	if !exists {
		return fmt.Errorf("Failed to find in state Managed Certificate %v", mcert.ObjectMeta.Name)
	}

	if sslCertificateState.Current == "" {
		return nil
	}

	sslCertificateName := sslCertificateState.New
	if sslCertificateName == "" {
		sslCertificateName = sslCertificateState.Current
	}

	sslCert, err := c.sslClient.Get(sslCertificateName)
	if err != nil {
		return err
	}

	switch sslCert.Managed.Status {
	case sslActive:
		mcert.Status.CertificateStatus = "Active"
	case sslManagedCertificateStatusUnspecified, "":
		mcert.Status.CertificateStatus = ""
	case sslProvisioning:
		mcert.Status.CertificateStatus = "Provisioning"
	case sslProvisioningFailed:
		mcert.Status.CertificateStatus = "ProvisioningFailed"
	case sslProvisioningFailedPermanently:
		mcert.Status.CertificateStatus = "ProvisioningFailedPermanently"
	case sslRenewalFailed:
		mcert.Status.CertificateStatus = "RenewalFailed"
	default:
		return fmt.Errorf("Unexpected status %v of SslCertificate %v", sslCert.Managed.Status, sslCert)
	}

	var domainStatus []api.DomainStatus
	// [review] domainStatus := make([]api.DomainStatus, 0)
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
	mcert.Status.DomainStatus = domainStatus
	mcert.Status.CertificateName = sslCert.Name

	_, err = c.client.AlphaV1alpha1().ManagedCertificates(mcert.ObjectMeta.Namespace).Update(mcert)
	return err
}

// [review] all the operations that reach out to gcp would be good to have more logging here

func (c *McertController) updateSslCertificate(mcert *api.ManagedCertificate) error {
	sslCertificateState, exists := c.state.Get(mcert.ObjectMeta.Name)
	if !exists {
		return fmt.Errorf("Failed to find in state Managed Certificate %v", mcert.ObjectMeta.Name)
	}

	if sslCertificateState.New == "" {
		newName, err := c.randomName()
		if err != nil {
			return err
		}

		// [review] general comment: whenever possible, use more specific format (e.g. %q or %s instead of just %v). This applies everywhere in this codebase...
		glog.Infof("McertController adds to state new SslCertificate name %v for update of current SslCertificate %v associated with Managed Certificate %v", newName, sslCertificateState.Current, mcert.ObjectMeta.Name)
		sslCertificateState.New = newName
		c.state.PutState(mcert.ObjectMeta.Name, sslCertificateState)
	}

	if sslCert, err := c.sslClient.Get(sslCertificateState.New); err != nil {
		//New SslCertificate does not exist yet, create it
		glog.Infof("McertController creates a new SslCertificate %v for update of current SslCertificate %v associated with Managed Certificate %v", sslCertificateState.New, sslCertificateState.Current, mcert.ObjectMeta.Name)
		if err := c.sslClient.Insert(sslCertificateState.New, mcert.Spec.Domains); err != nil { // [review]
			return err
		}
		// [review] can we just short circuit here and return?
	} else { // nesting of else if else seems strange here, also potentially confusing. consider early returns or factoring into subfunctions
		if !utils.Equals(mcert, sslCert) || sslCert.Managed.Status == sslProvisioningFailedPermanently || sslCert.Managed.Status == sslRenewalFailed {
			//New SslCertificate exists, but is outdated or has a failure status, so remove it and create a new one
			if err := c.sslClient.Delete(sslCertificateState.New); err != nil { // [review]
				return err
			}
			if err = c.sslClient.Insert(sslCertificateState.New, mcert.Spec.Domains); err != nil { // [review]
				return err
			}
		} else if sslCert.Managed.Status == sslActive {
			// New SslCertificate exists and is active, so just make it the current one
			c.state.PutCurrent(mcert.ObjectMeta.Name, sslCertificateState.New)
		}
	}

	return nil
}

func (c *McertController) createSslCertificateIfNeeded(sslCertificateName string, mcert *api.ManagedCertificate) error { // [review]
	if sslCert, err := c.sslClient.Get(sslCertificateName); err != nil {
		//SslCertificate does not yet exist, create it
		glog.Infof("McertController creates a new SslCertificate %v associated with Managed Certificate %v, based on state", sslCertificateName, mcert.ObjectMeta.Name)
		err := c.sslClient.Insert(sslCertificateName, mcert.Spec.Domains)
		if err != nil {
			return err
		}
		// [review] use early return here
	} else {
		if !utils.Equals(mcert, sslCert) {
			//SslCertificate exists, but needs updating
			glog.Infof("McertController found inconsistent state between Managed Certificate %v (domains: %+v) and SslCertificate %v (domains: %+v), update is needed", mcert.ObjectMeta.Name, mcert.Spec.Domains, sslCert.Name, sslCert.Managed.Domains)
			return c.updateSslCertificate(mcert)
		}

		//SslCertificate exists and does not need updating, but maybe there is an ongoing update, that is no longer needed?
		if sslCertificateState, exists := c.state.Get(mcert.ObjectMeta.Name); !exists {
			return fmt.Errorf("Failed to in state Managed Certificate %v", mcert.ObjectMeta.Name)
		}

		if sslCertificateState.New != "" { // [review]
			//Ongoing update, now obsolete. Remove the new SslCertificate object.
			sslCertNameToDelete := sslCertificateState.New
			c.state.PutCurrent(mcert.ObjectMeta.Name, sslCertificateState.Current)
			glog.Infof("McertController stops no longer needed ongoing update of Managed Certificate %v. New SslCertificate %v removed from state", mcert.ObjectMeta.Name, sslCertNameToDelete)

			err := c.sslClient.Delete(sslCertNameToDelete)
			if err != nil {
				return err
			}

			glog.Infof("McertContoller deleted no longer needed SslCertificate %v that was created to update Managed Certificate %v", sslCertNameToDelete, mcert.ObjectMeta.Name)
		}
	}

	return nil
}

func (c *McertController) createSslCertificateNameIfNecessary(name string) (string, error) {
	sslCertificateState, exists := c.state.Get(name)
	if exists && sslCertificateState.Current != "" {
		return sslCertificateState.Current, nil
	}

	// [review] switch if order
	if !exists || sslCertificateState.Current == "" {
		//State does not have anything for this managed certificate or no SslCertificate is associated with it
		sslCertificateName, err := c.randomName()
		if err != nil {
			return "", err
		}

		glog.Infof("McertController adds to state new SslCertificate name %v associated with Managed Certificate %v", sslCertificateName, name)
		c.state.PutCurrent(name, sslCertificateName)
		return sslCertificateName, nil
	}

	return sslCertificateState.Current, nil
}

func (c *McertController) handleMcert(key string) error {
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	glog.Infof("McertController handling Managed Certificate %s:%s", ns, name)

	mcert, err := c.lister.ManagedCertificates(ns).Get(name)
	if err != nil {
		return err
	}

	sslCertificateName, err := c.createSslCertificateNameIfNecessary(name)
	if err != nil {
		return err
	}

	err = c.createSslCertificateIfNecessary(sslCertificateName, mcert)
	if err != nil {
		return err
	}

	return c.updateStatus(mcert)
}

func (c *McertController) processNext() bool {
	obj, shutdown := c.queue.Get()

	if shutdown {
		return false
	}

	// [review]: why is there a closure here?
	err := func(obj interface{}) error {
		defer c.queue.Done(obj)

		var key string
		var ok bool
		if key, ok = obj.(string); !ok {
			c.queue.Forget(obj)
			return fmt.Errorf("Expected string in mcertQueue but got %#v", obj) // [review]: %T prints the type
		}

		if err := c.handleMcert(key); err != nil {
			c.queue.AddRateLimited(obj)
			return err
		}

		c.queue.Forget(obj)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
	}

	return true
}

func (c *McertController) runWorker() {
	for c.processNext() {
		// [review] this seems incomplete
	}
}

// [review]: centralize random name ish code in its own file/package
// these should take a seed for repeatibility during testing
func (c *McertController) randomName() (string, error) {
	name, err := utils.RandomName()
	if err != nil {
		return "", err
	}

	_, err = c.sslClient.Get(name)
	if err == nil {
		//Name taken, choose a new one
		name, err = utils.RandomName()
		if err != nil {
			return "", err
		}
	}

	return name, nil
}
