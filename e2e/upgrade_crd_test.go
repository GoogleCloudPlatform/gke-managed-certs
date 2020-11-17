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
	"testing"
)

func TestUpgradeCRD(t *testing.T) {
	ctx := context.Background()

	for description, testCase := range map[string]struct {
		createResource func(ctx context.Context, name, domain string) error
	}{
		"v1beta1": {
			createResource: func(ctx context.Context, name, domain string) error {
				return clients.ManagedCertificate.CreateV1beta1(ctx, name, []string{domain})
			},
		},
		"v1beta2": {
			createResource: func(ctx context.Context, name, domain string) error {
				return clients.ManagedCertificate.CreateV1beta2(ctx, name, []string{domain})
			},
		},
	} {
		t.Run(description, func(t *testing.T) {
			if err := testCase.createResource(ctx, "upgrade-crd", "upgrade-crd1.example.com"); err != nil {
				t.Fatalf("Creation failed: %v", err)
			}

			mcrt, err := clients.ManagedCertificate.Get(ctx, "upgrade-crd")
			if err != nil {
				t.Fatal(err)
			}

			mcrt.Spec.Domains = append(mcrt.Spec.Domains, "upgrade-crd2.example.com")
			if err := clients.ManagedCertificate.Update(ctx, mcrt); err != nil {
				t.Fatalf("Failed to update %v", err)
			}

			if err := clients.ManagedCertificate.Delete(ctx, "upgrade-crd"); err != nil {
				t.Fatalf("Failed to delete %v", err)
			}
		})
	}
}
