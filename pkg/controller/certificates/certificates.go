/*
Copyright 2020 Google LLC

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

// Package certificates contains helper methods for performing operations on SslCertificate and ManagedCertificate objects.
package certificates

import (
	"fmt"
	"reflect"
	"sort"

	compute "google.golang.org/api/compute/v0.beta"

	apisv1beta2 "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/networking.gke.io/v1beta2"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/config"
)

// CopyStatus sets ManagedCertificate status based on SslCertificate object.
func CopyStatus(sslCert compute.SslCertificate, mcrt *apisv1beta2.ManagedCertificate, config *config.Config) error {
	certificateStatus, err := translateStatus(config.CertificateStatus.Certificate, sslCert.Managed.Status)
	if err != nil {
		return fmt.Errorf("Failed to translate status of SslCertificate %v, err: %s", sslCert, err.Error())
	}
	mcrt.Status.CertificateStatus = certificateStatus

	// Initialize with non-nil value to avoid ManagedCertificate CRD validation warnings
	domainStatuses := make([]apisv1beta2.DomainStatus, 0)
	for domain, status := range sslCert.Managed.DomainStatus {
		domainStatus, err := translateStatus(config.CertificateStatus.Domain, status)
		if err != nil {
			return err
		}

		domainStatuses = append(domainStatuses, apisv1beta2.DomainStatus{
			Domain: domain,
			Status: domainStatus,
		})
	}
	mcrt.Status.DomainStatus = domainStatuses

	mcrt.Status.CertificateName = sslCert.Name
	mcrt.Status.ExpireTime = sslCert.ExpireTime

	return nil
}

// Equal compares ManagedCertificate and SslCertificate for equality, i. e. if their domain sets are equal.
func Equal(mcrt apisv1beta2.ManagedCertificate, sslCert compute.SslCertificate) bool {
	mcrtDomains := make([]string, len(mcrt.Spec.Domains))
	copy(mcrtDomains, mcrt.Spec.Domains)
	sort.Strings(mcrtDomains)

	sslCertDomains := make([]string, len(sslCert.Managed.Domains))
	copy(sslCertDomains, sslCert.Managed.Domains)
	sort.Strings(sslCertDomains)

	return reflect.DeepEqual(mcrtDomains, sslCertDomains)
}

// translateStatus translates status based on statuses mappings.
func translateStatus(statuses map[string]string, status string) (string, error) {
	v, e := statuses[status]
	if !e {
		return "", fmt.Errorf("Unexpected status %s", status)
	}

	return v, nil
}
