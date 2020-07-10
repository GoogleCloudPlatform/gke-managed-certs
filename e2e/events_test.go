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

package e2e

import (
	"fmt"
	"regexp"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/GoogleCloudPlatform/gke-managed-certs/e2e/utils"
)

// This test creates more certificates than the quota allows and checks if there is at least one with
// an event communicating the Created event, and at least one with an event communicating TooManyCertificates.
// Normally every certificate should either receive Created or TooManyCertificates, however rarely BackendError
// can be reported as well, and events are reported on a best-effort basis, so the test does not require
// every event to be present.
func TestEvents(t *testing.T) {
	numCerts := 400 // Should be bigger than allowed quota.

	for i := 0; i < numCerts; i++ {
		name := fmt.Sprintf("quota-%d", i)
		if err := clients.ManagedCertificate.Create(name, []string{"quota.example.com"}); err != nil {
			t.Fatal(err)
		}
		defer clients.ManagedCertificate.Delete(name)
	}

	if err := utils.Retry(func() error {
		foundCreated := false
		foundQuotaExceeded := false

		eventList, err := clients.Event.List(metav1.ListOptions{})
		if err != nil {
			return err
		}

		for _, event := range eventList.Items {
			nameMatched, err := regexp.MatchString("quota-[0-9]+", event.Regarding.Name)
			if err != nil {
				return err
			}

			if event.Regarding.Kind == "ManagedCertificate" && nameMatched {
				if event.Reason == "Create" {
					foundCreated = true
				}
				if event.Reason == "TooManyCertificates" {
					foundQuotaExceeded = true
				}
			}

			if foundCreated && foundQuotaExceeded {
				break
			}
		}

		if !foundCreated || !foundQuotaExceeded {
			return fmt.Errorf("created event found: %t, quota exceeded event found: %t; want both found", foundCreated, foundQuotaExceeded)
		}

		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
