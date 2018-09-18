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

// Package translate translates SslCertificate statuses to ManagedCertificate statuses.
package translate

import (
	"fmt"
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

// DomainStatus translates an SslCertificate domain status to ManagedCertificate domain status.
func DomainStatus(status string) (string, error) {
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

// Status translates an SslCertificate status to ManagedCertificate status.
func Status(status string) (string, error) {
	switch status {
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
		return "", fmt.Errorf("Unexpected status %s", status)
	}
}
