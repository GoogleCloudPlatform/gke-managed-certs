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
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/klog"

	"github.com/GoogleCloudPlatform/gke-managed-certs/e2e/utils"
	utilshttp "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/http"
)

const (
	additionalSslCertificateDomain = "example.com"
	annotation                     = "networking.gke.io/managed-certificates"
	annotationSeparator            = ","
	maxNameLength                  = 15
	port                           = 8080
	statusActive                   = "Active"
	statusSuccess                  = 200
)

func mustCreateBackendService(t *testing.T, name string) {
	t.Helper()

	if err := utilshttp.IgnoreNotFound(clients.Deployment.Delete(name, &metav1.DeleteOptions{})); err != nil {
		t.Fatal(err)
	}
	klog.Infof("Deleted deployment %s", name)

	appHello := map[string]string{"app": name}
	args := []string{
		"-e",
		fmt.Sprintf("require('http').createServer(function (req, res) { res.end('Hello world!'); }).listen(%d);", port),
	}

	depl := &extv1beta1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: extv1beta1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: appHello},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: appHello},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyAlways,
					Containers: []corev1.Container{
						{
							Name:    "http-hello",
							Image:   "node:11-slim",
							Command: []string{"node"},
							Args:    args,
							Ports:   []corev1.ContainerPort{{ContainerPort: port}},
						},
					},
				},
			},
		},
	}
	if _, err := clients.Deployment.Create(depl); err != nil {
		t.Fatal(err)
	}
	klog.Infof("Created deployment %s", name)

	if err := utilshttp.IgnoreNotFound(clients.Service.Delete(name, &metav1.DeleteOptions{})); err != nil {
		t.Fatal(err)
	}
	klog.Infof("Deleted service %s", name)

	serv := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeNodePort,
			Ports:    []corev1.ServicePort{{Port: port}},
			Selector: appHello,
		},
	}
	if _, err := clients.Service.Create(serv); err != nil {
		t.Fatal(err)
	}
	klog.Infof("Created service %s", name)
}

func mustCreateIngress(t *testing.T, name string) {
	t.Helper()

	if err := utilshttp.IgnoreNotFound(clients.Ingress.Delete(name, &metav1.DeleteOptions{})); err != nil {
		t.Fatal(err)
	}
	klog.Infof("Deleted ingress %s", name)

	ing := &extv1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: extv1beta1.IngressSpec{
			Backend: &extv1beta1.IngressBackend{
				ServiceName: "http-hello",
				ServicePort: intstr.FromInt(port),
			},
		},
	}
	if _, err := clients.Ingress.Create(ing); err != nil {
		t.Fatal(err)
	}
	klog.Infof("Created ingress %s", name)
}

func getIngressIP(name string) (string, error) {
	var ip string
	err := utils.Retry(func() error {
		ing, err := clients.Ingress.Get(name, metav1.GetOptions{})
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

	backendServiceName := "http-hello"
	mustCreateBackendService(t, backendServiceName)
	defer func() {
		clients.Deployment.Delete(backendServiceName, &metav1.DeleteOptions{})
		clients.Service.Delete(backendServiceName, &metav1.DeleteOptions{})
	}()

	ingressName := "test-workflow-ingress"
	mustCreateIngress(t, ingressName)
	defer clients.Ingress.Delete(ingressName, &metav1.DeleteOptions{})

	ip, err := getIngressIP(ingressName)
	if err != nil {
		t.Fatal(err)
	}
	klog.Infof("Ingress IP: %s", ip)

	defer clients.Dns.DeleteAll()
	domains, err := clients.Dns.Create(generateRandomNames(2), ip)
	if err != nil {
		t.Fatal(err)
	}
	klog.Infof("Generated random domains: %v", domains)

	var mcrtNames []string
	for i, domain := range domains {
		mcrtName := fmt.Sprintf("provisioning-workflow-%d", i)
		mcrtNames = append(mcrtNames, mcrtName)
		err := clients.ManagedCertificate.Create(mcrtName, []string{domain})
		if err != nil {
			t.Fatal(err)
		}
	}
	klog.Infof("Created ManagedCertficate resources: %s", mcrtNames)

	additionalSslCertificateName := fmt.Sprintf("additional-%s", generateRandomNames(1)[0])
	if err := clients.SslCertificate.Create(ctx, additionalSslCertificateName,
		[]string{additionalSslCertificateDomain}); err != nil {
		t.Fatal(err)
	}
	defer clients.SslCertificate.Delete(ctx, additionalSslCertificateName)
	klog.Infof("Created additional SslCertificate resource: %s", additionalSslCertificateName)

	ing, err := clients.Ingress.Get(ingressName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	ing.Annotations[annotation] = strings.Join(mcrtNames, annotationSeparator)
	if _, err := clients.Ingress.Update(ing); err != nil {
		t.Fatal(err)
	}
	klog.Infof("Annotated Ingress with %s=%s", annotation, ing.Annotations[annotation])

	t.Run("ManagedCertificate resources attached to Ingress become Active", func(t *testing.T) {
		err := utils.Retry(func() error {
			for _, mcrtName := range mcrtNames {
				mcrt, err := clients.ManagedCertificate.Get(mcrtName)
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
