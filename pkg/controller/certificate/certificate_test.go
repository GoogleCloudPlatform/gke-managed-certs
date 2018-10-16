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

package certificate

import (
	"reflect"
	"testing"

	compute "google.golang.org/api/compute/v0.beta"

	api "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/gke.googleapis.com/v1alpha1"
)

const (
	fakeNameFieldValue = "name"
	fakeTimeFieldValue = "time"
)

func sslCert(status string, domains map[string]string) compute.SslCertificate {
	return compute.SslCertificate{
		Name:       fakeNameFieldValue,
		ExpireTime: fakeTimeFieldValue,
		Managed: &compute.SslCertificateManagedSslCertificate{
			Status:       status,
			DomainStatus: domains,
		},
	}
}

func mcrt(status string, domains []api.DomainStatus) *api.ManagedCertificate {
	return &api.ManagedCertificate{
		Status: api.ManagedCertificateStatus{
			CertificateName:   fakeNameFieldValue,
			CertificateStatus: status,
			ExpireTime:        fakeTimeFieldValue,
			DomainStatus:      domains,
		},
	}
}

func TestCopyStatus(t *testing.T) {
	testCases := []struct {
		sslCertIn compute.SslCertificate
		success   bool // translation should succeed
		mcrtOut   *api.ManagedCertificate
		desc      string
	}{
		{sslCert("bad_status", nil), false, nil, "Wrong certificate status"},
		{sslCert("ACTIVE", map[string]string{"example.com": "bad_status"}), false, nil, "Wrong domain status"},
		{sslCert("ACTIVE", nil), true, mcrt("Active", []api.DomainStatus{}), "Nil domain statuses -> []{} domain status"},
		{sslCert("ACTIVE", map[string]string{"example.com": "ACTIVE"}), true, mcrt("Active", []api.DomainStatus{api.DomainStatus{Domain: "example.com", Status: "Active"}}), "Correct translation"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.desc, func(t *testing.T) {
			var mcrt api.ManagedCertificate
			err := CopyStatus(testCase.sslCertIn, &mcrt)

			if (err == nil) != testCase.success {
				t.Errorf("Translation err: %s, want success: %t", err.Error(), testCase.success)
			}

			if err != nil {
				return
			}

			if !reflect.DeepEqual(&mcrt, testCase.mcrtOut) {
				t.Errorf("ManagedCertificate after Certificate(%#v) = %v, want %v", testCase.sslCertIn, mcrt, testCase.mcrtOut)
			}
		})
	}
}

func TestEqual(t *testing.T) {
	var testCases = []struct {
		mcrtDomains    []string
		sslCertDomains []string
		expected       bool
		desc           string
	}{
		{nil, nil, true, "nil == nil"},
		{[]string{}, []string{}, true, "[] == []"},
		{nil, []string{}, true, "nil == []"},
		{[]string{}, nil, true, "[] == nil"},
		{[]string{"a"}, nil, false, "[a] != nil"},
		{[]string{"a"}, []string{}, false, "[a] != []"},
		{[]string{"a"}, []string{"b"}, false, "[a] != [b]"},
		{[]string{"a", "b"}, []string{"b", "a"}, true, "[a, b] == [b, a]"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.desc, func(t *testing.T) {
			mcrt := api.ManagedCertificate{
				Spec: api.ManagedCertificateSpec{
					Domains: testCase.mcrtDomains,
				},
			}
			sslCert := compute.SslCertificate{
				Managed: &compute.SslCertificateManagedSslCertificate{
					Domains: testCase.sslCertDomains,
				},
				Type: "MANAGED",
			}

			if result := Equal(mcrt, sslCert); result != testCase.expected {
				t.Errorf("Equal(mcrt, sslCert) = %t, want %t. ManagedCertificate: %v, SslCertificate: %v", result, testCase.expected, mcrt, sslCert)
			}
		})
	}
}
