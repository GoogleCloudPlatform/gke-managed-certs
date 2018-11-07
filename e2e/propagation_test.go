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

	"github.com/GoogleCloudPlatform/gke-managed-certs/e2e/client"
	"github.com/GoogleCloudPlatform/gke-managed-certs/e2e/utils"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/certificates"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/http"
)

func ensurePropagated(t *testing.T, client *client.Clients, name string) error {
	return utils.Retry(func() error {
		mcrt, err := client.ManagedCertificate.Get(namespace, name)
		if err != nil {
			return err
		}

		if mcrt.Status.CertificateName == "" {
			return fmt.Errorf("SslCertificate name empty in status of %s:%s", namespace, name)
		}

		sslCert, err := client.SslCertificate.Get(mcrt.Status.CertificateName)
		if err != nil {
			return err
		}

		if !certificates.Equal(*mcrt, *sslCert) {
			return fmt.Errorf("Certificates different, want equal domains; ManagedCertificate: %v, SslCertificate: %v", mcrt, sslCert)
		}

		return nil
	})
}

func TestPropagation(t *testing.T) {
	t.Parallel()

	type testCase struct {
		action func(client *client.Clients, mcrtName string) error
		desc   string
	}

	testCases := []testCase{
		{
			func(client *client.Clients, mcrtName string) error {
				mcrt, err := client.ManagedCertificate.Get(namespace, mcrtName)
				if err != nil {
					return err
				}

				if mcrt.Status.CertificateName == "" {
					return fmt.Errorf("SslCertificate name empty in status of %s:%s", namespace, mcrtName)
				}

				return client.SslCertificate.Delete(mcrt.Status.CertificateName)
			},
			"Deleted SslCertificate is recreated",
		},
		{
			func(client *client.Clients, mcrtName string) error {
				mcrt, err := client.ManagedCertificate.Get(namespace, mcrtName)
				if err != nil {
					return err
				}

				mcrt.Spec.Domains = []string{"foo.com"}
				return client.ManagedCertificate.Update(mcrt)
			},
			"Modifications in ManagedCertificate are propagated to SslCertificate",
		},
	}

	client := utils.Setup(t)
	defer utils.TearDown(t, client)

	for i, tc := range testCases {
		go func(i int, tc testCase) {
			t.Run(tc.desc, func(t *testing.T) {
				name := fmt.Sprintf("propagation-%d", i)
				domain := fmt.Sprintf("example-%d.com", i)
				if err := client.ManagedCertificate.Create(namespace, name, []string{domain}); err != nil {
					t.Fatalf("Creation failed: %s", err.Error())
				}

				if err := ensurePropagated(t, client, name); err != nil {
					t.Fatalf("Propagation failed: %s", err.Error())
				}

				if err := tc.action(client, name); err != nil {
					t.Fatalf("Action failed: %s", err.Error())
				}

				if err := ensurePropagated(t, client, name); err != nil {
					t.Errorf("Propagation after action failed: %s", err.Error())
				}
			})
		}(i, tc)
	}

	t.Run("Deleting ManagedCertificate deletes SslCertificate", func(t *testing.T) {
		name := "propagation-to-be-deleted"
		domain := "example-to-be-deleted.com"
		if err := client.ManagedCertificate.Create(namespace, name, []string{domain}); err != nil {
			t.Fatalf("Creation failed: %s", err.Error())
		}

		if err := ensurePropagated(t, client, name); err != nil {
			t.Fatalf("Propagation failed: %s", err.Error())
		}

		mcrt, err := client.ManagedCertificate.Get(namespace, name)
		if err != nil {
			t.Fatalf("%s", err.Error())
		}

		if err := client.ManagedCertificate.Delete(namespace, name); err != nil {
			t.Fatalf("%s", err.Error())
		}

		err = utils.Retry(func() error {
			_, err := client.SslCertificate.Get(mcrt.Status.CertificateName)
			if http.IsNotFound(err) {
				return nil
			}

			return fmt.Errorf("SslCertificate %s exists, want deleted", mcrt.Status.CertificateName)
		})
		if err != nil {
			t.Errorf("%s", err.Error())
		}
	})
}
