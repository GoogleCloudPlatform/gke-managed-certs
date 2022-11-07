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

package ssl

import (
	"context"

	computev1 "google.golang.org/api/compute/v1"

	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/errors"
)

func newFakeError(code string) *Error {
	return &Error{
		operation: &computev1.Operation{
			Error: &computev1.OperationError{
				Errors: []*computev1.OperationErrorErrors{{Code: code}},
			},
		},
	}
}

func NewFakeQuotaExceededError() *Error {
	return newFakeError(codeQuotaExceeded)
}

func NewFakeSslCertificate(name, status string, domainToStatus map[string]string) *computev1.SslCertificate {
	var domains []string
	for domain := range domainToStatus {
		domains = append(domains, domain)
	}

	return &computev1.SslCertificate{
		Managed: &computev1.SslCertificateManagedSslCertificate{
			Domains:      domains,
			Status:       status,
			DomainStatus: domainToStatus,
		},
		Name: name,
		Type: typeManaged,
	}
}

// Fake
type Fake struct {
	mapping map[string]*computev1.SslCertificate
}

var _ Interface = &Fake{}

func (f *Fake) Create(ctx context.Context, name string, domains []string) error {
	domainToStatus := make(map[string]string, 0)
	for _, domain := range domains {
		domainToStatus[domain] = ""
	}
	f.mapping[name] = NewFakeSslCertificate(name, "", domainToStatus)
	return nil
}

func (f *Fake) Delete(ctx context.Context, name string) error {
	if _, e := f.mapping[name]; !e {
		return errors.NotFound
	}

	delete(f.mapping, name)
	return nil
}

func (f *Fake) Get(name string) (*computev1.SslCertificate, error) {
	if _, e := f.mapping[name]; !e {
		return nil, errors.NotFound
	}

	return f.mapping[name], nil
}

func (f *Fake) List() ([]*computev1.SslCertificate, error) {
	var result []*computev1.SslCertificate
	for _, v := range f.mapping {
		result = append(result, v)
	}
	return result, nil
}

// Builder
type Builder struct {
	fake *Fake
}

func NewFake() *Builder {
	return &Builder{
		fake: &Fake{mapping: make(map[string]*computev1.SslCertificate, 0)},
	}
}

func (b *Builder) AddEntry(name string, domains []string) *Builder {
	domainToStatus := make(map[string]string, 0)
	for _, domain := range domains {
		domainToStatus[domain] = ""
	}
	b.fake.mapping[name] = NewFakeSslCertificate(name, "", domainToStatus)
	return b
}

func (b *Builder) AddEntryWithStatus(name, status string, domainToStatus map[string]string) *Builder {
	b.fake.mapping[name] = NewFakeSslCertificate(name, status, domainToStatus)
	return b
}

func (b *Builder) Build() Interface {
	return b.fake
}
