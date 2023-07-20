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

package certificates

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	computev1 "google.golang.org/api/compute/v1"

	v1 "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/networking.gke.io/v1"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/config"
)

const (
	fakeNameFieldValue = "name"
	fakeTimeFieldValue = "time"
)

func sslCert(status string, domains map[string]string) computev1.SslCertificate {
	return computev1.SslCertificate{
		Name:       fakeNameFieldValue,
		ExpireTime: fakeTimeFieldValue,
		Managed: &computev1.SslCertificateManagedSslCertificate{
			Status:       status,
			DomainStatus: domains,
		},
	}
}

func mcrt(status string, domains []v1.DomainStatus) *v1.ManagedCertificate {
	return &v1.ManagedCertificate{
		Status: v1.ManagedCertificateStatus{
			CertificateName:   fakeNameFieldValue,
			CertificateStatus: status,
			ExpireTime:        fakeTimeFieldValue,
			DomainStatus:      domains,
		},
	}
}

func TestCopyStatus(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		sslCertIn   computev1.SslCertificate
		wantSuccess bool // translation should succeed
		wantMcrt    *v1.ManagedCertificate
	}{
		"Wrong certificate status": {
			sslCert("bad_status", nil),
			false,
			nil,
		},
		"Wrong domain status": {
			sslCert("ACTIVE", map[string]string{"example.com": "bad_status"}),
			false,
			nil,
		},
		"Nil domain statuses -> []{} domain status": {
			sslCert("ACTIVE", nil),
			true,
			mcrt("Active", []v1.DomainStatus{}),
		},
		"Correct translation": {
			sslCert("ACTIVE", map[string]string{"example.com": "ACTIVE"}),
			true,
			mcrt("Active", []v1.DomainStatus{{Domain: "example.com", Status: "Active"}}),
		},
		"Correct translation, domainStatus in sorted order": {
			sslCert("ACTIVE", map[string]string{
				"a.example.com": "ACTIVE",
				"c.example.com": "ACTIVE",
				"b.example.com": "ACTIVE",
			}),
			true,
			mcrt("Active", []v1.DomainStatus{
				{Domain: "a.example.com", Status: "Active"},
				{Domain: "b.example.com", Status: "Active"},
				{Domain: "c.example.com", Status: "Active"},
			}),
		},
	}

	for description, testCase := range testCases {
		testCase := testCase
		t.Run(description, func(t *testing.T) {
			t.Parallel()

			var mcrt v1.ManagedCertificate
			err := CopyStatus(testCase.sslCertIn, &mcrt, config.NewFake())

			if (err == nil) != testCase.wantSuccess {
				t.Errorf("Translation err: %v, want success: %t",
					err, testCase.wantSuccess)
			}

			if err != nil {
				return
			}

			if diff := cmp.Diff(testCase.wantMcrt, &mcrt); diff != "" {
				t.Errorf("CopyStatus, diff ManagedCertificate (-want, +got): %s",
					diff)
			}
		})
	}
}

func TestDiff(t *testing.T) {
	t.Parallel()

	var testCases = map[string]struct {
		mcrtDomains    []string
		sslCertDomains []string
		wantEmptyDiff  bool
	}{
		"nil == nil":       {nil, nil, true},
		"[] == []":         {[]string{}, []string{}, true},
		"nil == []":        {nil, []string{}, true},
		"[] == nil":        {[]string{}, nil, true},
		"[a] != nil":       {[]string{"a"}, nil, false},
		"[a] != []":        {[]string{"a"}, []string{}, false},
		"[a] != [b]":       {[]string{"a"}, []string{"b"}, false},
		"[a, b] == [b, a]": {[]string{"a", "b"}, []string{"b", "a"}, true},
	}

	for description, testCase := range testCases {
		testCase := testCase
		t.Run(description, func(t *testing.T) {
			t.Parallel()

			mcrt := v1.ManagedCertificate{
				Spec: v1.ManagedCertificateSpec{
					Domains: testCase.mcrtDomains,
				},
			}
			sslCert := computev1.SslCertificate{
				Managed: &computev1.SslCertificateManagedSslCertificate{
					Domains: testCase.sslCertDomains,
				},
				Type: "MANAGED",
			}

			if diff := Diff(mcrt, sslCert); (diff == "") != testCase.wantEmptyDiff {
				t.Errorf(`Diff(ManagedCertificate, SslCertificate) = %s,
					want empty diff: %t`, diff, testCase.wantEmptyDiff)
			}
		})
	}
}
