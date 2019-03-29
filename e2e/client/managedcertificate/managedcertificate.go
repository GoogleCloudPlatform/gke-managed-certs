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

package managedcertificate

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"

	api "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/networking.gke.io/v1beta1"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/clientset/versioned"
	clientset "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/clientset/versioned/typed/networking.gke.io/v1beta1"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/http"
)

type ManagedCertificate interface {
	Create(name string, domains []string) error
	DeleteAll() error
	Delete(name string) error
	Get(name string) (*api.ManagedCertificate, error)
	Update(mcrt *api.ManagedCertificate) error
}

type managedCertificateImpl struct {
	// client manages ManagedCertificate custom resources
	client clientset.ManagedCertificateInterface
}

func New(config *rest.Config, namespace string) (ManagedCertificate, error) {
	clientset, err := versioned.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return managedCertificateImpl{
		client: clientset.NetworkingV1beta1().ManagedCertificates(namespace),
	}, nil
}

func (m managedCertificateImpl) Create(name string, domains []string) error {
	mcrt := &api.ManagedCertificate{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: api.ManagedCertificateSpec{
			Domains: domains,
		},
		Status: api.ManagedCertificateStatus{
			DomainStatus: []api.DomainStatus{},
		},
	}
	_, err := m.client.Create(mcrt)
	return err
}

func (m managedCertificateImpl) DeleteAll() error {
	mcrts, err := m.client.List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, mcrt := range mcrts.Items {
		if err := m.Delete(mcrt.Name); err != nil {
			return err
		}
	}

	return nil
}

func (m managedCertificateImpl) Delete(name string) error {
	return http.IgnoreNotFound(m.client.Delete(name, &metav1.DeleteOptions{}))
}

func (m managedCertificateImpl) Get(name string) (*api.ManagedCertificate, error) {
	return m.client.Get(name, metav1.GetOptions{})
}

func (m managedCertificateImpl) Update(mcrt *api.ManagedCertificate) error {
	_, err := m.client.Update(mcrt)
	return err
}
