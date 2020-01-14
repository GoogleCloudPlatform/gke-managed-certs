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
	"context"
	"fmt"
	"testing"

	"github.com/GoogleCloudPlatform/gke-managed-certs/e2e/utils"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/certificates"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/http"
)

func ensurePropagated(name string) error {
	return utils.Retry(func() error {
		mcrt, err := clients.ManagedCertificate.Get(name)
		if err != nil {
			return err
		}

		if mcrt.Status.CertificateName == "" {
			return fmt.Errorf("SslCertificate name empty in status of %s", name)
		}

		sslCert, err := clients.SslCertificate.Get(mcrt.Status.CertificateName)
		if err != nil {
			return err
		}

		if !certificates.Equal(*mcrt, *sslCert) {
			return fmt.Errorf("Certificates have different domains, want same; %v, %v",
				mcrt.Spec.Domains, sslCert.Managed.Domains)
		}

		return nil
	})
}

func TestPropagation(t *testing.T) {
	for i, tc := range []struct {
		action func(mcrtName string) error
		desc   string
	}{
		{
			func(mcrtName string) error {
				mcrt, err := clients.ManagedCertificate.Get(mcrtName)
				if err != nil {
					return err
				}

				if mcrt.Status.CertificateName == "" {
					return fmt.Errorf("SslCertificate name empty in status of %s", mcrtName)
				}

				return clients.SslCertificate.Delete(context.Background(), mcrt.Status.CertificateName)
			},
			"Deleted SslCertificate is recreated",
		},
		{
			func(mcrtName string) error {
				mcrt, err := clients.ManagedCertificate.Get(mcrtName)
				if err != nil {
					return err
				}

				mcrt.Spec.Domains = []string{"propagation-rename.example.com"}
				return clients.ManagedCertificate.Update(mcrt)
			},
			"Modifications in ManagedCertificate are propagated to SslCertificate",
		},
	} {
		i, tc := i, tc
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()
			name := fmt.Sprintf("propagation-%d", i)
			domain := fmt.Sprintf("propagation-%d.example.com", i)
			if err := clients.ManagedCertificate.Create(name, []string{domain}); err != nil {
				t.Fatalf("Creation failed: %s", err.Error())
			}

			if err := ensurePropagated(name); err != nil {
				t.Fatalf("Propagation failed: %s", err.Error())
			}

			if err := tc.action(name); err != nil {
				t.Fatalf("Action failed: %s", err.Error())
			}

			if err := ensurePropagated(name); err != nil {
				t.Fatalf("Propagation after action failed: %s", err.Error())
			}
		})
	}

	t.Run("Deleting ManagedCertificate deletes SslCertificate", func(t *testing.T) {
		name := "propagation-to-be-deleted"
		domain := "propagation-to-be-deleted.example.com"
		if err := clients.ManagedCertificate.Create(name, []string{domain}); err != nil {
			t.Fatal(err)
		}

		if err := ensurePropagated(name); err != nil {
			t.Fatal(err)
		}

		mcrt, err := clients.ManagedCertificate.Get(name)
		if err != nil {
			t.Fatal(err)
		}

		if err := clients.ManagedCertificate.Delete(name); err != nil {
			t.Fatal(err)
		}

		err = utils.Retry(func() error {
			_, err := clients.SslCertificate.Get(mcrt.Status.CertificateName)
			if http.IsNotFound(err) {
				return nil
			}

			return fmt.Errorf("SslCertificate %s exists, want deleted", mcrt.Status.CertificateName)
		})
		if err != nil {
			t.Fatal(err)
		}
	})
}
