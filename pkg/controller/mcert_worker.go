package controller

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	api "managed-certs-gke/pkg/apis/cloud.google.com/v1alpha1"
)

const (
	maxNameLength = 63
)

func translateDomainStatus(status string) (string, error) {
	switch status {
	case "PROVISIONING":
		return "Provisioning",nil
	case "FAILED_NOT_VISIBLE":
		return "FailedNotVisible", nil
	case "FAILED_CAA_CHECKING":
		return "FailedCaaChecking", nil
	case "FAILED_CAA_FORBIDDEN":
		return "FailedCaaForbidden", nil
	case "FAILED_RATE_LIMITED":
		return "FailedRateLimited", nil
	case "ACTIVE":
		return "Active", nil
	default:
		return "", fmt.Errorf("Unexpected status %v", status)
	}
}

func (c *McertController) updateStatus(mcert *api.ManagedCertificate) error {
	sslCert, err := c.sslClient.Get(mcert.ObjectMeta.Name)
	if err != nil {
		return err
	}

	switch sslCert.Managed.Status {
	case "ACTIVE":
		mcert.Status.CertificateStatus = "Active"
	case "MANAGED_CERTIFICATE_STATUS_UNSPECIFIED":
		mcert.Status.CertificateStatus = ""
	case "PROVISIONING":
		mcert.Status.CertificateStatus = "Provisioning"
	case "PROVISIONING_FAILED":
		mcert.Status.CertificateStatus = "ProvisioningFailed"
	case "PROVISIONING_FAILED_PERMANENTLY":
		mcert.Status.CertificateStatus = "ProvisioningFailedPermanently"
	case "RENEWAL_FAILED":
		mcert.Status.CertificateStatus = "RenewalFailed"
	default:
		return fmt.Errorf("Unexpected status %v of SslCertificate %v", sslCert.Managed.Status, sslCert)
	}

	var domainStatus []api.DomainStatus
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

	_, err = c.client.CloudV1alpha1().ManagedCertificates(mcert.ObjectMeta.Namespace).Update(mcert)
	return err
}

func (c *McertController) createSslCertificateIfNecessary(mcert *api.ManagedCertificate) error {
	sslCertificateName, exists := c.state.Get(mcert.ObjectMeta.Name)
	if !exists {
		return fmt.Errorf("There should be a name for SslCertificate associated with ManagedCertificate %v, but it is missing", mcert.ObjectMeta.Name)
	}

	sslCert, err := c.sslClient.Get(sslCertificateName)
	glog.Infof("Tried to fetch sslCert with name %v, result: %v, err: %v", sslCertificateName, sslCert, err)

	if err != nil {
		//SslCertificate does not yet exist, create it
		glog.Infof("Create a new SslCertificate %v associated with ManagedCertificate %v", sslCertificateName, mcert.ObjectMeta.Name)
		err := c.sslClient.Insert(sslCertificateName, mcert.Spec.Domains)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *McertController) createSslCertificateNameIfNecessary(name string) error {
	sslCertificateName, exists := c.state.Get(name)
	if !exists || sslCertificateName == "" {
		//State does not have anything for this managed certificate or no SslCertificate is associated with it
		sslCertificateName, err := c.getRandomName()
		if err != nil {
			return err
		}

		glog.Infof("Add new SslCertificate name %v associated with ManagedCertificate %v", sslCertificateName, name)
		c.state.Put(name, sslCertificateName)
	}

	return nil
}

func (c *McertController) handleMcert(key string) error {
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	glog.Infof("Handling ManagedCertificate %s.%s", ns, name)

	mcert, err := c.lister.ManagedCertificates(ns).Get(name)
	if err != nil {
		return err
	}

	err = c.createSslCertificateNameIfNecessary(name)
	if err != nil {
		return err
	}

	err = c.createSslCertificateIfNecessary(mcert)
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

	err := func(obj interface{}) error {
		defer c.queue.Done(obj)

		var key string
		var ok bool
		if key, ok = obj.(string); !ok {
			c.queue.Forget(obj)
			return fmt.Errorf("Expected string in mcertQueue but got %#v", obj)
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
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}

	return b
}

func createRandomName() (string, error) {
	uid, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}

	generatedName := fmt.Sprintf("mcert%s", uid.String())
	maxLength := min(len(generatedName), maxNameLength)
	return generatedName[:maxLength], nil
}

func (c *McertController) getRandomName() (string, error) {
	name, err := createRandomName()
	if err != nil {
		return "", err
	}

	_, err = c.sslClient.Get(name)
	if err == nil {
		//Name taken, choose a new one
		name, err = createRandomName()
		if err != nil {
			return "", err
		}
	}

	return name, nil
}
