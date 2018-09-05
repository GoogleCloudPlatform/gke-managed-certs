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

package utils

import (
	"testing"

	compute "google.golang.org/api/compute/v0.alpha"

	api "managed-certs-gke/pkg/apis/gke.googleapis.com/v1alpha1"
)

func newMcert(domains []string) *api.ManagedCertificate {
	return &api.ManagedCertificate{
		Spec: api.ManagedCertificateSpec{
			Domains: domains,
		},
	}
}

func newSslCert(domains []string) *compute.SslCertificate {
	return &compute.SslCertificate{
		Managed: &compute.SslCertificateManagedSslCertificate{
			Domains: domains,
		},
	}
}

func TestEquals_emptyObjects(t *testing.T) {
	mcert := newMcert([]string{})
	sslCert := newSslCert([]string{})

	if !Equals(mcert, sslCert) {
		t.Errorf("Empty objects should be equal")
	}
}

func TestEquals_differentLengths(t *testing.T) {
	mcert := newMcert([]string{"abc"})
	sslCert := newSslCert([]string{})

	if Equals(mcert, sslCert) {
		t.Errorf("Objects of different length should not be equal")
	}

	if len(mcert.Spec.Domains) != 1 || mcert.Spec.Domains[0] != "abc" || len(sslCert.Managed.Domains) != 0 {
		t.Errorf("Equals modified passed objects")
	}
}

func TestEquals_differentOrder(t *testing.T) {
	mcert := newMcert([]string{"abc", "def"})
	sslCert := newSslCert([]string{"def", "abc"})

	if !Equals(mcert, sslCert) {
		t.Errorf("Objects of different order should be equal")
	}

	if len(mcert.Spec.Domains) != 2 || mcert.Spec.Domains[0] != "abc" || mcert.Spec.Domains[1] != "def" || len(sslCert.Managed.Domains) != 2 || sslCert.Managed.Domains[0] != "def" || sslCert.Managed.Domains[1] != "abc" {
		t.Errorf("Equals modified passed objects")
	}
}
