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
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/GoogleCloudPlatform/gke-managed-certs/e2e/client"
	api "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/gke.googleapis.com/v1alpha1"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/http"
)

const (
	namespace = "default"
)

func TestValidateCRD(t *testing.T) {
	testCases := []struct {
		domains []string
		success bool
		desc    string
	}{
		{[]string{"a.com", "b.com"}, false, "Multiple domain names not allowed"},
		{[]string{"very-long-domain-name-which-exceeds-the-limit-of-63-characters.com"}, false, "Domain >63 chars not allowed"},
		{[]string{"*.example.com"}, false, "Domain with a wildcard not allowed"},
		{[]string{"example.com"}, true, "Single non-wildcard domain <=63 characters allowed"},
	}

	client, err := client.New()
	if err != nil {
		t.Fatalf("Could not create client %s", err.Error())
	}

	for i, testCase := range testCases {
		t.Run(testCase.desc, func(t *testing.T) {
			mcrt := &api.ManagedCertificate{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("test-case-%d", i),
				},
				Spec: api.ManagedCertificateSpec{
					Domains: testCase.domains,
				},
				Status: api.ManagedCertificateStatus{
					DomainStatus: []api.DomainStatus{},
				},
			}

			nsClient := client.Mcrt.GkeV1alpha1().ManagedCertificates(namespace)
			_, err := nsClient.Create(mcrt)
			if err == nil && !testCase.success {
				t.Errorf("Created, want failure")
				return
			}
			if err == nil && testCase.success {
				// Creation succeeded as expected, so now delete the managed certificate
				if err := http.IgnoreNotFound(nsClient.Delete(mcrt.Name, &metav1.DeleteOptions{})); err != nil {
					t.Error(err)
				}

				return
			}

			statusErr, ok := err.(*errors.StatusError)
			if !ok {
				t.Errorf("Creation failed with error %T, want errors.StatusError. Error: %s", err, err.Error())
				return
			}

			if statusErr.Status().Reason != "Invalid" {
				t.Errorf("Creation failed with reason %s, want Invalid, Error: %#v", statusErr.Status().Reason, err)
			}
		})
	}
}
