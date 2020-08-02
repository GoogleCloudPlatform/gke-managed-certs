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

package managedcertificate

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"

	apisv1 "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/networking.gke.io/v1"
	apisv1beta1 "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/networking.gke.io/v1beta1"
	apisv1beta2 "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/networking.gke.io/v1beta2"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/clientset/versioned"
	clientsetv1 "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/clientset/versioned/typed/networking.gke.io/v1"
	clientsetv1beta1 "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/clientset/versioned/typed/networking.gke.io/v1beta1"
	clientsetv1beta2 "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/clientset/versioned/typed/networking.gke.io/v1beta2"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/errors"
)

type ManagedCertificate interface {
	CreateV1beta1(name string, domains []string) error
	CreateV1beta2(name string, domains []string) error
	Create(name string, domains []string) error
	DeleteAll() error
	Delete(name string) error
	Get(name string) (*apisv1.ManagedCertificate, error)
	Update(mcrt *apisv1.ManagedCertificate) error
}

type managedCertificateImpl struct {
	// clientv1beta1 manages ManagedCertificate v1beta1 custom resources
	clientv1beta1 clientsetv1beta1.ManagedCertificateInterface
	// clientv1beta2 manages ManagedCertificate v1beta2 custom resources
	clientv1beta2 clientsetv1beta2.ManagedCertificateInterface
	// client manages ManagedCertificate custom resources
	client clientsetv1.ManagedCertificateInterface
}

func New(config *rest.Config, namespace string) (ManagedCertificate, error) {
	clientset, err := versioned.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return managedCertificateImpl{
		clientv1beta1: clientset.NetworkingV1beta1().ManagedCertificates(namespace),
		clientv1beta2: clientset.NetworkingV1beta2().ManagedCertificates(namespace),
		client:        clientset.NetworkingV1().ManagedCertificates(namespace),
	}, nil
}

func (m managedCertificateImpl) CreateV1beta1(name string, domains []string) error {
	mcrt := &apisv1beta1.ManagedCertificate{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: apisv1beta1.ManagedCertificateSpec{
			Domains: domains,
		},
		Status: apisv1beta1.ManagedCertificateStatus{
			DomainStatus: []apisv1beta1.DomainStatus{},
		},
	}
	_, err := m.clientv1beta1.Create(mcrt)
	return err
}

func (m managedCertificateImpl) CreateV1beta2(name string, domains []string) error {
	mcrt := &apisv1beta2.ManagedCertificate{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: apisv1beta2.ManagedCertificateSpec{
			Domains: domains,
		},
		Status: apisv1beta2.ManagedCertificateStatus{
			DomainStatus: []apisv1beta2.DomainStatus{},
		},
	}
	_, err := m.clientv1beta2.Create(mcrt)
	return err
}

func (m managedCertificateImpl) Create(name string, domains []string) error {
	mcrt := &apisv1.ManagedCertificate{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: apisv1.ManagedCertificateSpec{
			Domains: domains,
		},
		Status: apisv1.ManagedCertificateStatus{
			DomainStatus: []apisv1.DomainStatus{},
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
	return errors.IgnoreNotFound(m.client.Delete(name, &metav1.DeleteOptions{}))
}

func (m managedCertificateImpl) Get(name string) (*apisv1.ManagedCertificate, error) {
	return m.client.Get(name, metav1.GetOptions{})
}

func (m managedCertificateImpl) Update(mcrt *apisv1.ManagedCertificate) error {
	_, err := m.client.Update(mcrt)
	return err
}
