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
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/http"
)

type ManagedCertificate interface {
	Create(namespace, name string, domains []string) error
	DeleteAll(namespace string) error
	Delete(namespace, name string) error
	Get(namespace, name string) (*api.ManagedCertificate, error)
	Update(mcrt *api.ManagedCertificate) error
}

type managedCertificateImpl struct {
	// clientset manages ManagedCertificate custom resources
	clientset versioned.Interface
}

func New(config *rest.Config) (ManagedCertificate, error) {
	clientset, err := versioned.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return managedCertificateImpl{
		clientset: clientset,
	}, nil
}

func (m managedCertificateImpl) Create(namespace, name string, domains []string) error {
	nsClient := m.clientset.NetworkingV1beta1().ManagedCertificates(namespace)
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
	_, err := nsClient.Create(mcrt)
	return err
}

func (m managedCertificateImpl) DeleteAll(namespace string) error {
	mcrts, err := m.clientset.NetworkingV1beta1().ManagedCertificates(namespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, mcrt := range mcrts.Items {
		if err := m.Delete(namespace, mcrt.Name); err != nil {
			return err
		}
	}

	return nil
}

func (m managedCertificateImpl) Delete(namespace, name string) error {
	return http.IgnoreNotFound(m.clientset.NetworkingV1beta1().ManagedCertificates(namespace).Delete(name, &metav1.DeleteOptions{}))
}

func (m managedCertificateImpl) Get(namespace, name string) (*api.ManagedCertificate, error) {
	return m.clientset.NetworkingV1beta1().ManagedCertificates(namespace).Get(name, metav1.GetOptions{})
}

func (m managedCertificateImpl) Update(mcrt *api.ManagedCertificate) error {
	_, err := m.clientset.NetworkingV1beta1().ManagedCertificates(mcrt.Namespace).Update(mcrt)
	return err
}
