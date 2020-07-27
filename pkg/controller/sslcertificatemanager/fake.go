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

package sslcertificatemanager

import (
	"context"

	compute "google.golang.org/api/compute/v1"

	apisv1 "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/networking.gke.io/v1"
)

const (
	typeManaged = "MANAGED"
)

type Fake struct {
	mapping map[string]*compute.SslCertificate
}

var _ SslCertificateManager = &Fake{}

func NewFake() *Fake {
	return &Fake{mapping: make(map[string]*compute.SslCertificate, 0)}
}

func NewFakeWithEntry(sslCertificateName string, domains []string, status string,
	domainStatus []string) *Fake {

	manager := NewFake()
	manager.mapping[sslCertificateName] = newSslCertificate(sslCertificateName,
		domains, status, domainStatus)

	return manager
}

func newSslCertificate(name string, domains []string, status string,
	domainStatus []string) *compute.SslCertificate {

	domainToStatus := make(map[string]string, 0)
	for i := 0; i < len(domains) && i < len(domainStatus); i++ {
		domainToStatus[domains[i]] = domainStatus[i]
	}

	return &compute.SslCertificate{
		Managed: &compute.SslCertificateManagedSslCertificate{
			Domains:      domains,
			Status:       status,
			DomainStatus: domainToStatus,
		},
		Name: name,
		Type: typeManaged,
	}
}

func (f *Fake) Create(ctx context.Context, sslCertificateName string,
	managedCertificate apisv1.ManagedCertificate) error {

	f.mapping[sslCertificateName] = newSslCertificate(sslCertificateName,
		managedCertificate.Spec.Domains, "", nil)
	return nil
}

func (f *Fake) Delete(ctx context.Context, sslCertificateName string,
	managedCertificate *apisv1.ManagedCertificate) error {

	delete(f.mapping, sslCertificateName)
	return nil
}

func (f *Fake) Exists(sslCertificateName string,
	managedCertificate *apisv1.ManagedCertificate) (bool, error) {

	_, exists := f.mapping[sslCertificateName]
	return exists, nil
}

func (f *Fake) Get(sslCertificateName string,
	managedCertificate *apisv1.ManagedCertificate) (*compute.SslCertificate, error) {

	sslCert := f.mapping[sslCertificateName]
	return sslCert, nil
}
