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

package e2e

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/GoogleCloudPlatform/gke-managed-certs/e2e/utils"
	api "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/gke.googleapis.com/v1alpha1"
)

const (
	namespace = "default"
)

func TestValidateCRD(t *testing.T) {
	testCases := []struct {
		domains []string
		desc    string
	}{
		{[]string{"a.com", "b.com"}, "Multiple domain names not allowed"},
		{[]string{"very-long-domain-name-which-exceeds-the-limit-of-63-characters.com"}, "Domain >63 characters not allowed."},
		{[]string{"*.example.com"}, "Domain with a wildcard not allowed"},
	}

	client, err := utils.CreateManagedCertificateClient()
	if err != nil {
		t.Fatalf("Could not create client %s", err.Error())
	}

	for _, testCase := range testCases {
		t.Run(testCase.desc, func(t *testing.T) {
			mcrt := &api.ManagedCertificate{
				Spec: api.ManagedCertificateSpec{
					Domains: testCase.domains,
				},
			}
			_, err := client.GkeV1alpha1().ManagedCertificates(namespace).Create(mcrt)
			if err == nil {
				t.Errorf("ManagedCertificate created, want failure")
			}

			if _, ok := err.(*errors.StatusError); !ok {
				t.Errorf("ManagedCertificate creation failed with error %T, want apimachinery errors.StatusError. Error details: %#v", err, err)
			}

			// errors.StatusError is expected - to indicate validation error
		})
	}
}
