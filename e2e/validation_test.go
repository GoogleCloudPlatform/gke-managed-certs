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

	"github.com/GoogleCloudPlatform/gke-managed-certs/e2e/utils"
)

func TestCRDValidation(t *testing.T) {
	t.Parallel()

	type testCase struct {
		domains []string
		success bool
		desc    string
	}

	testCases := []testCase{
		{[]string{"a.com", "b.com"}, false, "Multiple domain names not allowed"},
		{[]string{"very-long-domain-name-which-exceeds-the-limit-of-63-characters.com"}, false, "Domain >63 chars not allowed"},
		{[]string{"*.example.com"}, false, "Domain with a wildcard not allowed"},
		{[]string{"example.com"}, true, "Single non-wildcard domain <=63 characters allowed"},
	}

	client := utils.Setup(t)
	defer utils.TearDown(t, client)

	for i, tc := range testCases {
		i, tc := i, tc
		t.Run(tc.desc, func(t *testing.T) {
			name := fmt.Sprintf("crd-validation-%d", i)
			err := client.ManagedCertificate.Create(namespace, name, tc.domains)
			if err == nil && !tc.success {
				t.Fatalf("Created, want failure")
			}
			if err == nil && tc.success {
				// Creation succeeded as expected, so now delete the managed certificate
				if err := client.ManagedCertificate.Delete(namespace, name); err != nil {
					t.Error(err)
				}

				return
			}

			statusErr, ok := err.(*errors.StatusError)
			if !ok {
				t.Fatalf("Creation failed with error %T, want errors.StatusError. Error: %s", err, err.Error())
			}

			if statusErr.Status().Reason != "Invalid" {
				t.Errorf("Creation failed with reason %s, want Invalid, Error: %#v", statusErr.Status().Reason, err)
			}
		})
	}
}
