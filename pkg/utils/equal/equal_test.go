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

package equal

import (
	"testing"

	compute "google.golang.org/api/compute/v0.alpha"
	api "managed-certs-gke/pkg/apis/gke.googleapis.com/v1alpha1"
)

var testCases = []struct {
	mcertDomains   []string
	sslCertDomains []string
	equal          bool
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

func TestAre(t *testing.T) {
	for _, testCase := range testCases {
		t.Run(testCase.desc, func(t *testing.T) {
			mcert := api.ManagedCertificate{
				Spec: api.ManagedCertificateSpec{
					Domains: testCase.mcertDomains,
				},
			}
			sslCert := compute.SslCertificate{
				Managed: &compute.SslCertificateManagedSslCertificate{
					Domains: testCase.sslCertDomains,
				},
				Type: "MANAGED",
			}

			if Are(mcert, sslCert) != testCase.equal {
				t.Errorf("Are(%v, %v) expected to be %t", testCase.mcertDomains, testCase.sslCertDomains, testCase.equal)
			}
		})
	}
}
