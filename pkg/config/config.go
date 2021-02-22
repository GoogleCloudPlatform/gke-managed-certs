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

// Package config manages configuration of the whole application.
package config

import (
	"context"
	"fmt"
	"os"
	"time"

	"cloud.google.com/go/compute/metadata"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	compute "google.golang.org/api/compute/v1"
	gcfg "gopkg.in/gcfg.v1"
	"k8s.io/klog"
	"k8s.io/legacy-cloud-providers/gce"
)

const (
	managedActive                        = "Active"
	managedEmpty                         = ""
	managedFailedCaaChecking             = "FailedCaaChecking"
	managedFailedCaaForbidden            = "FailedCaaForbidden"
	managedFailedNotVisible              = "FailedNotVisible"
	managedFailedRateLimited             = "FailedRateLimited"
	managedProvisioning                  = "Provisioning"
	managedProvisioningFailed            = "ProvisioningFailed"
	managedProvisioningFailedPermanently = "ProvisioningFailedPermanently"
	managedRenewalFailed                 = "RenewalFailed"

	sslActive                              = "ACTIVE"
	sslEmpty                               = ""
	sslFailedCaaChecking                   = "FAILED_CAA_CHECKING"
	sslFailedCaaForbidden                  = "FAILED_CAA_FORBIDDEN"
	sslFailedNotVisible                    = "FAILED_NOT_VISIBLE"
	sslFailedRateLimited                   = "FAILED_RATE_LIMITED"
	sslManagedCertificateStatusUnspecified = "MANAGED_CERTIFICATE_STATUS_UNSPECIFIED"
	sslProvisioning                        = "PROVISIONING"
	sslProvisioningFailed                  = "PROVISIONING_FAILED"
	sslProvisioningFailedPermanently       = "PROVISIONING_FAILED_PERMANENTLY"
	sslRenewalFailed                       = "RENEWAL_FAILED"

	AnnotationManagedCertificatesKey = "networking.gke.io/managed-certificates"
	AnnotationPreSharedCertKey       = "ingress.gcp.kubernetes.io/pre-shared-cert"
	SslCertificateNamePrefix         = "mcrt-"
)

type computeConfig struct {
	TokenSource oauth2.TokenSource
	ProjectID   string
	Timeout     time.Duration
}

type certificateStatusConfig struct {
	// Certificate is a mapping from SslCertificate status to ManagedCertificate status
	Certificate map[string]string
	// Domain is a mapping from SslCertificate domain status to ManagedCertificate domain status
	Domain map[string]string
}

type masterElectionConfig struct {
	// Maximum duration that a leader can be stopped before it is replaced by another candidate
	LeaseDuration time.Duration
	// Interval between attempts by an acting master to renew a leadership slot
	RenewDeadline time.Duration
	// Duration the clients should wait between attempting acquisition and renewal of a leadership
	RetryPeriod time.Duration
}

type Config struct {
	// CertificateStatus holds mappings of SslCertificate statuses to ManagedCertificate statuses
	CertificateStatus certificateStatusConfig
	// Compute is GCP-specific configuration
	Compute computeConfig
	// MasterElection holds configuration settings needed for electing master
	MasterElection masterElectionConfig
	// SslCertificateNamePrefix is a prefix prepended to SslCertificate resources created by the controller
	SslCertificateNamePrefix string
}

func New(ctx context.Context, gceConfigFilePath string) (*Config, error) {
	tokenSource, projectID, err := getTokenSourceAndProjectID(ctx, gceConfigFilePath)
	if err != nil {
		return nil, err
	}

	klog.Infof("TokenSource: %#v, projectID: %s", tokenSource, projectID)

	domainStatuses := make(map[string]string, 0)
	domainStatuses[sslActive] = managedActive
	domainStatuses[sslFailedCaaChecking] = managedFailedCaaChecking
	domainStatuses[sslFailedCaaForbidden] = managedFailedCaaForbidden
	domainStatuses[sslFailedNotVisible] = managedFailedNotVisible
	domainStatuses[sslFailedRateLimited] = managedFailedRateLimited
	domainStatuses[sslProvisioning] = managedProvisioning

	certificateStatuses := make(map[string]string, 0)
	certificateStatuses[sslActive] = managedActive
	certificateStatuses[sslEmpty] = managedEmpty
	certificateStatuses[sslManagedCertificateStatusUnspecified] = managedEmpty
	certificateStatuses[sslProvisioning] = managedProvisioning
	certificateStatuses[sslProvisioningFailed] = managedProvisioningFailed
	certificateStatuses[sslProvisioningFailedPermanently] = managedProvisioningFailedPermanently
	certificateStatuses[sslRenewalFailed] = managedRenewalFailed

	return &Config{
		CertificateStatus: certificateStatusConfig{
			Certificate: certificateStatuses,
			Domain:      domainStatuses,
		},
		Compute: computeConfig{
			TokenSource: tokenSource,
			ProjectID:   projectID,
			Timeout:     30 * time.Second,
		},
		MasterElection: masterElectionConfig{
			LeaseDuration: 15 * time.Second,
			RenewDeadline: 10 * time.Second,
			RetryPeriod:   2 * time.Second,
		},
		SslCertificateNamePrefix: SslCertificateNamePrefix,
	}, nil
}

func getTokenSourceAndProjectID(ctx context.Context, gceConfigFilePath string) (oauth2.TokenSource, string, error) {
	if gceConfigFilePath != "" {
		klog.V(1).Info("In a GKE cluster")

		config, err := os.Open(gceConfigFilePath)
		if err != nil {
			return nil, "", fmt.Errorf("Could not open cloud provider configuration %s: %v", gceConfigFilePath, err)
		}
		defer config.Close()

		var cfg gce.ConfigFile
		if err := gcfg.FatalOnly(gcfg.ReadInto(&cfg, config)); err != nil {
			return nil, "", fmt.Errorf("Could not read config %v", err)
		}
		klog.Infof("Using GCE provider config %+v", cfg)

		return gce.NewAltTokenSource(cfg.Global.TokenURL, cfg.Global.TokenBody), cfg.Global.ProjectID, nil
	}

	projectID, err := metadata.ProjectID()
	if err != nil {
		return nil, "", fmt.Errorf("Could not fetch project id: %v", err)
	}

	if len(os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")) > 0 {
		klog.V(1).Info("In a GCP cluster")
		tokenSource, err := google.DefaultTokenSource(ctx, compute.ComputeScope)
		return tokenSource, projectID, err
	} else {
		klog.V(1).Info("Using default TokenSource")
		return google.ComputeTokenSource(""), projectID, nil
	}
}
