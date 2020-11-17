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
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"

	"github.com/GoogleCloudPlatform/gke-managed-certs/e2e/utils"
)

const (
	additionalSslCertificateDomain = "example.com"
	annotationSeparator            = ","
	maxNameLength                  = 15
	statusActive                   = "Active"
	statusSuccess                  = 200
)

func getIngressIP(ctx context.Context, name string) (string, error) {
	var ip string
	err := utils.Retry(func() error {
		ing, err := clients.Ingress.Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		lbIngresses := ing.Status.LoadBalancer.Ingress
		if len(lbIngresses) > 0 && lbIngresses[0].IP != "" {
			ip = lbIngresses[0].IP
			return nil
		}

		return fmt.Errorf("Could not get Ingress IP")
	})

	return ip, err
}

func generateRandomNames(count int) []string {
	var result []string

	for ; count > 0; count-- {
		randomName := uuid.New().String()
		maxLength := len(randomName)
		if maxLength > maxNameLength {
			maxLength = maxNameLength
		}
		result = append(result, randomName[:maxLength])
	}

	return result
}

func TestProvisioningWorkflow(t *testing.T) {
	ctx := context.Background()

	mcrtCount := 2
	var mcrtNames []string
	for i := 0; i < mcrtCount; i++ {
		mcrtNames = append(mcrtNames, fmt.Sprintf("test-provisioning-workflow-%d", i))
	}

	ingressName := "test-provisioning-workflow"
	createIngress(t, ctx, ingressName, 8080, strings.Join(mcrtNames, annotationSeparator))

	ip, err := getIngressIP(ctx, ingressName)
	if err != nil {
		t.Fatal(err)
	}
	klog.Infof("Ingress IP: %s", ip)

	domains, records, err := clients.Dns.Create(generateRandomNames(2*mcrtCount), ip)
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Dns.Delete(records)
	klog.Infof("Generated random domains: %v", domains)

	for i, mcrtName := range mcrtNames {
		err := clients.ManagedCertificate.Create(ctx, mcrtName, []string{domains[2*i], domains[2*i+1]})
		if err != nil {
			t.Fatal(err)
		}
		defer clients.ManagedCertificate.Delete(ctx, mcrtName)
	}

	additionalSslCertificateName := fmt.Sprintf("additional-%s", generateRandomNames(1)[0])
	if err := clients.SslCertificate.Create(ctx, additionalSslCertificateName,
		[]string{additionalSslCertificateDomain}); err != nil {
		t.Fatalf("Failed to create additional SslCertificate %s: %v", additionalSslCertificateName, err)
	}
	defer clients.SslCertificate.Delete(ctx, additionalSslCertificateName)
	klog.Infof("Created additional SslCertificate resource: %s", additionalSslCertificateName)

	t.Run("ManagedCertificate resources attached to Ingress become Active", func(t *testing.T) {
		err := utils.Retry(func() error {
			for _, mcrtName := range mcrtNames {
				mcrt, err := clients.ManagedCertificate.Get(ctx, mcrtName)
				if err != nil {
					return err
				}

				if mcrt.Status.CertificateStatus != statusActive {
					return fmt.Errorf("ManagedCertificate not yet active: %#v", mcrt)
				}
			}

			return nil
		})
		if err != nil {
			t.Fatal(err)
		}

		t.Run("HTTPS requests succeed", func(t *testing.T) {
			err := utils.Retry(func() error {
				for _, domain := range domains {
					response, err := http.Get(fmt.Sprintf("https://%s", domain))
					if err != nil {
						return err
					}
					defer response.Body.Close()

					if response.StatusCode != statusSuccess {
						return fmt.Errorf("HTTP GET to %s returned status code %d, want %d", domain, response.StatusCode, statusSuccess)
					}
				}

				return nil
			})
			if err != nil {
				t.Fatal(err)
			}

			t.Run("Additional SslCertificate is not modified", func(t *testing.T) {
				sslCertificate, err := clients.SslCertificate.Get(additionalSslCertificateName)
				if err != nil {
					t.Fatal(err)
				}

				sslCertDomains := sslCertificate.Managed.Domains
				if len(sslCertDomains) != 1 || sslCertDomains[0] != additionalSslCertificateDomain {
					t.Fatalf("Additional SslCertificate domains: %v, want a single %s", sslCertDomains, additionalSslCertificateDomain)
				}
			})
		})
	})
}
