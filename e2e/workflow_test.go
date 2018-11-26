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
	"net/http"
	"strings"
	"testing"

	"github.com/golang/glog"
	"github.com/google/uuid"

	"github.com/GoogleCloudPlatform/gke-managed-certs/e2e/utils"
)

const (
	additionalSslCertificateDomain = "example.com"
	annotation                     = "gke.googleapis.com/managed-certificates"
	annotationSeparator            = ","
	maxNameLength                  = 25
	statusActive                   = "Active"
	statusSuccess                  = 200
)

func getIngressIP(name string) (string, error) {
	var ip string
	err := utils.Retry(func() error {
		ing, err := clients.Ingress.Get(namespace, name)
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
	t.Parallel()

	ingressName := "test-workflow-ingress"
	if err := clients.Ingress.Delete(namespace, ingressName); err != nil {
		t.Fatal(err)
	}
	glog.Infof("Deleted ingress %s:%s", namespace, ingressName)

	if err := clients.Ingress.Create(namespace, ingressName); err != nil {
		t.Fatal(err)
	}
	defer clients.Ingress.Delete(namespace, ingressName)
	glog.Infof("Created ingress %s:%s", namespace, ingressName)

	ip, err := getIngressIP(ingressName)
	if err != nil {
		t.Fatal(err)
	}
	glog.Infof("Ingress IP: %s", ip)

	defer clients.Dns.DeleteAll()
	domains, err := clients.Dns.Create(generateRandomNames(2), ip)
	if err != nil {
		t.Fatal(err)
	}
	glog.Infof("Generated random domains: %v", domains)

	var mcrtNames []string
	for i, domain := range domains {
		mcrtName := fmt.Sprintf("provisioning-workflow-%d", i)
		mcrtNames = append(mcrtNames, mcrtName)
		err := clients.ManagedCertificate.Create(namespace, mcrtName, []string{domain})
		if err != nil {
			t.Fatal(err)
		}
	}
	glog.Infof("Created ManagedCertficate resources: %s", mcrtNames)

	additionalSslCertificateName := fmt.Sprintf("additional-%s", generateRandomNames(1)[0])
	if err := clients.SslCertificate.Create(additionalSslCertificateName, []string{additionalSslCertificateDomain}); err != nil {
		t.Fatal(err)
	}
	defer clients.SslCertificate.Delete(additionalSslCertificateName)
	glog.Infof("Created additional SslCertificate resource: %s", additionalSslCertificateName)

	ing, err := clients.Ingress.Get(namespace, ingressName)
	if err != nil {
		t.Fatal(err)
	}
	ing.Annotations[annotation] = strings.Join(mcrtNames, annotationSeparator)
	if err := clients.Ingress.Update(ing); err != nil {
		t.Fatal(err)
	}
	glog.Infof("Annotated Ingress with %s=%s", annotation, ing.Annotations[annotation])

	t.Run("ManagedCertificate resources attached to Ingress become Active", func(t *testing.T) {
		err := utils.Retry(func() error {
			for _, mcrtName := range mcrtNames {
				mcrt, err := clients.ManagedCertificate.Get(namespace, mcrtName)
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
