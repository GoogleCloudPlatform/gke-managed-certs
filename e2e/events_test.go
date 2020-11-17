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
	"context"
	"fmt"
	"regexp"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/GoogleCloudPlatform/gke-managed-certs/e2e/utils"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/errors"
)

// This test creates more certificates than the quota allows and checks if there is at least one with
// an event communicating the Created event, and at least one with an event communicating TooManyCertificates.
// Normally every certificate should either receive Created or TooManyCertificates, however rarely BackendError
// can be reported as well, and events are reported on a best-effort basis, so the test does not require
// every event to be present.
func TestEvents_ManagedCertificate(t *testing.T) {
	ctx := context.Background()
	numCerts := 400 // Should be bigger than allowed quota.

	for i := 0; i < numCerts; i++ {
		name := fmt.Sprintf("quota-%d", i)
		if err := errors.IgnoreNotFound(clients.ManagedCertificate.Delete(ctx, name)); err != nil {
			t.Fatal(err)
		}
		if err := clients.ManagedCertificate.Create(ctx, name, []string{"quota.example.com"}); err != nil {
			t.Fatal(err)
		}
		defer clients.ManagedCertificate.Delete(ctx, name)
	}

	if err := utils.Retry(func() error {
		foundCreated := false
		foundTooManyCertificates := false

		eventList, err := clients.Event.List(ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}

		for _, event := range eventList.Items {
			nameMatched, err := regexp.MatchString("quota-[0-9]+", event.InvolvedObject.Name)
			if err != nil {
				return err
			}

			if event.InvolvedObject.Kind == "ManagedCertificate" && nameMatched {
				if event.Reason == "Create" {
					foundCreated = true
				}
				if event.Reason == "TooManyCertificates" {
					foundTooManyCertificates = true
				}
			}

			if foundCreated && foundTooManyCertificates {
				break
			}
		}

		if !foundCreated || !foundTooManyCertificates {
			return fmt.Errorf("Create event found: %t, TooManyCertificates event found: %t; want both found", foundCreated, foundTooManyCertificates)
		}

		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestEvents_Ingress(t *testing.T) {
	ctx := context.Background()
	ingressName := "test-events-ingress"

	if err := createIngress(t, ctx, ingressName, 8081, "non-existing-certificate"); err != nil {
		t.Fatalf("createIngress(ingressName=%s): %v", ingressName, err)
	}

	if err := utils.Retry(func() error {
		eventList, err := clients.Event.List(ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}

		for _, event := range eventList.Items {
			if event.InvolvedObject.Kind == "Ingress" && event.InvolvedObject.Name == ingressName && event.Reason == "MissingCertificate" {
				return nil
			}
		}

		return fmt.Errorf("MissingCertificate event not found")
	}); err != nil {
		t.Fatal(err)
	}
}
