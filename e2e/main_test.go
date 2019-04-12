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
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/golang/glog"
	compute "google.golang.org/api/compute/v0.beta"
	apiextv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/GoogleCloudPlatform/gke-managed-certs/e2e/client"
	"github.com/GoogleCloudPlatform/gke-managed-certs/e2e/utils"
	utilshttp "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/http"
)

const (
	namespace   = "default"
	platformEnv = "PLATFORM"
)

var clients *client.Clients

func TestMain(m *testing.M) {
	flag.Parse()

	var err error
	clients, err = client.New(namespace)
	if err != nil {
		glog.Fatalf("Could not create clients: %s", err.Error())
	}

	platform := os.Getenv(platformEnv)
	glog.Infof("platform=%s", platform)
	gke := (strings.ToLower(platform) == "gke")

	sslCertificatesBegin, err := setUp(clients, gke)
	if err != nil {
		glog.Fatal(err)
	}

	exitCode := m.Run()

	if err := tearDown(clients, gke, sslCertificatesBegin); err != nil {
		glog.Fatal(err)
	}

	os.Exit(exitCode)
}

func createManagedCertificateCRD() error {
	domainRegex := `^(([a-zA-Z0-9]+|[a-zA-Z0-9][-a-zA-Z0-9]*[a-zA-Z0-9])\.)+[a-zA-Z][-a-zA-Z0-9]*[a-zA-Z0-9]\.?$`
	var maxDomains int64 = 1
	var maxDomainLength int64 = 63
	crd := apiextv1beta1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CustomResourceDefinition",
			APIVersion: "apiextensions.k8s.io/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "managedcertificates.networking.gke.io",
		},
		Spec: apiextv1beta1.CustomResourceDefinitionSpec{
			Group:   "networking.gke.io",
			Version: "v1beta1",
			Names: apiextv1beta1.CustomResourceDefinitionNames{
				Plural:     "managedcertificates",
				Singular:   "managedcertificate",
				Kind:       "ManagedCertificate",
				ShortNames: []string{"mcrt"},
			},
			Scope: apiextv1beta1.NamespaceScoped,
			Validation: &apiextv1beta1.CustomResourceValidation{
				OpenAPIV3Schema: &apiextv1beta1.JSONSchemaProps{
					Properties: map[string]apiextv1beta1.JSONSchemaProps{
						"status": {
							Properties: map[string]apiextv1beta1.JSONSchemaProps{
								"certificateStatus": {Type: "string"},
								"domainStatus": {
									Type: "array",
									Items: &apiextv1beta1.JSONSchemaPropsOrArray{
										Schema: &apiextv1beta1.JSONSchemaProps{
											Type:     "object",
											Required: []string{"domain", "status"},
											Properties: map[string]apiextv1beta1.JSONSchemaProps{
												"domain": {Type: "string"},
												"status": {Type: "string"},
											},
										},
									},
								},
								"certificateName": {Type: "string"},
								"expireTime":      {Type: "string", Format: "date-time"},
							},
						},
						"spec": {
							Properties: map[string]apiextv1beta1.JSONSchemaProps{
								"domains": {
									Type:     "array",
									MaxItems: &maxDomains,
									Items: &apiextv1beta1.JSONSchemaPropsOrArray{
										Schema: &apiextv1beta1.JSONSchemaProps{
											Type:      "string",
											MaxLength: &maxDomainLength,
											Pattern:   domainRegex,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	if _, err := clients.CustomResource.Create(&crd); err != nil {
		return err
	}
	glog.Infof("Created custom resource definition %s", crd.Name)

	if err := utils.Retry(func() error {
		crd, err := clients.CustomResource.Get(crd.Name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("ManagedCertificate CRD not yet established: %v", err)
		}

		for _, c := range crd.Status.Conditions {
			if c.Type == apiextv1beta1.Established && c.Status == apiextv1beta1.ConditionTrue {
				return nil
			}
		}

		return errors.New("ManagedCertificate CRD not yet established")
	}); err != nil {
		return err
	}

	return nil
}

func setUp(clients *client.Clients, gke bool) ([]*compute.SslCertificate, error) {
	glog.Info("setting up")

	if !gke {
		if err := createManagedCertificateCRD(); err != nil {
			return nil, err
		}
	}

	if err := clients.ManagedCertificate.DeleteAll(); err != nil {
		return nil, err
	}

	sslCertificatesBegin, err := clients.SslCertificate.List()
	if err != nil {
		return nil, err
	}

	glog.Info("set up success")
	return sslCertificatesBegin, nil
}

func tearDown(clients *client.Clients, gke bool, sslCertificatesBegin []*compute.SslCertificate) error {
	glog.Infof("tearing down")

	if err := clients.ManagedCertificate.DeleteAll(); err != nil {
		return err
	}

	if err := utils.Retry(func() error {
		sslCertificatesEnd, err := clients.SslCertificate.List()
		if err != nil {
			return err
		}

		if added, removed, equal := diff(sslCertificatesBegin, sslCertificatesEnd); !equal {
			return fmt.Errorf("Waiting for SslCertificates clean up. + %v - %v, want both empty", added, removed)
		}

		return nil
	}); err != nil {
		return err
	}

	if !gke {
		name := "managedcertificates.networking.gke.io"
		if err := utilshttp.IgnoreNotFound(clients.CustomResource.Delete(name, &metav1.DeleteOptions{})); err != nil {
			return err
		}
		glog.Infof("Deleted custom resource definition %s", name)
	}

	glog.Infof("tear down success")
	return nil
}

func diff(begin, end []*compute.SslCertificate) ([]string, []string, bool) {
	var added, removed []string

	for _, b := range begin {
		found := false

		for _, e := range end {
			if b.Name == e.Name {
				found = true
				break
			}
		}

		if !found {
			removed = append(removed, b.Name)
		}
	}

	for _, e := range end {
		found := false

		for _, b := range begin {
			if e.Name == b.Name {
				found = true
				break
			}
		}

		if !found {
			added = append(added, e.Name)
		}
	}

	return added, removed, len(added) == 0 && len(removed) == 0
}
