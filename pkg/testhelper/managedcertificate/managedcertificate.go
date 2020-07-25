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
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"

	apisv1beta2 "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/networking.gke.io/v1beta2"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/clientset/versioned/fake"
	listersv1beta2 "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/listers/networking.gke.io/v1beta2"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/types"
)

var (
	groupKind     = schema.GroupKind{Group: "networking.gke.io", Kind: "ManagedCertificate"}
	groupResource = schema.GroupResource{Group: "networking.gke.io", Resource: "ManagedCertificate"}
)

type builder struct {
	managedCertificate *apisv1beta2.ManagedCertificate
}

// New builds a ManagedCertificate for a given domain and id.
func New(id types.CertId, domain string) *builder {
	return &builder{
		&apisv1beta2.ManagedCertificate{
			ObjectMeta: metav1.ObjectMeta{
				CreationTimestamp: metav1.Now().Rfc3339Copy(),
				Namespace:         id.Namespace,
				Name:              id.Name,
			},
			Spec: apisv1beta2.ManagedCertificateSpec{
				Domains: []string{domain},
			},
			Status: apisv1beta2.ManagedCertificateStatus{
				CertificateStatus: "",
				DomainStatus:      []apisv1beta2.DomainStatus{},
			},
		},
	}
}

func (b *builder) WithStatus(status string, domainStatus ...string) *builder {
	b.managedCertificate.Status.CertificateStatus = status
	for i, domain := range b.managedCertificate.Spec.Domains {
		b.managedCertificate.Status.DomainStatus = append(b.managedCertificate.Status.DomainStatus, apisv1beta2.DomainStatus{
			Domain: domain,
			Status: domainStatus[i],
		})
	}
	return b
}

func (b *builder) WithCertificateName(certificateName string) *builder {
	b.managedCertificate.Status.CertificateName = certificateName
	return b
}

func (b *builder) Build() *apisv1beta2.ManagedCertificate {
	return b.managedCertificate
}

// Clientset implements the ManagedCertificate Clientset interface and overrides the Update method,
type Clientset struct {
	fake.Clientset

	managedCertificates []*apisv1beta2.ManagedCertificate
}

func NewClientset(managedCertificates []*apisv1beta2.ManagedCertificate) *Clientset {
	return &Clientset{managedCertificates: managedCertificates}
}

func (c *Clientset) Update(managedCertificate *apisv1beta2.ManagedCertificate) (*apisv1beta2.ManagedCertificate, error) {
	for i, cert := range c.managedCertificates {
		if cert.Namespace == managedCertificate.Namespace && cert.Name == managedCertificate.Name {
			c.managedCertificates[i] = managedCertificate
			return managedCertificate, nil
		}
	}

	return nil, errors.NewNotFound(groupResource, managedCertificate.Name)
}

// Lister implements the ManagedCertificate Lister interface.
type Lister struct {
	managedCertificates []*apisv1beta2.ManagedCertificate
}

var _ listersv1beta2.ManagedCertificateLister = &Lister{}

func NewLister(managedCertificates []*apisv1beta2.ManagedCertificate) *Lister {
	return &Lister{managedCertificates: managedCertificates}
}

func (l *Lister) List(selector labels.Selector) ([]*apisv1beta2.ManagedCertificate, error) {
	return l.managedCertificates, nil
}

func (l *Lister) ManagedCertificates(namespace string) listersv1beta2.ManagedCertificateNamespaceLister {
	return &namespacedLister{
		managedCertificates: l.managedCertificates,
		namespace:           namespace,
	}
}

// namespacedLister implements the ManagedCertificate namespaced Lister interface.
type namespacedLister struct {
	managedCertificates []*apisv1beta2.ManagedCertificate
	namespace           string
}

var _ listersv1beta2.ManagedCertificateNamespaceLister = &namespacedLister{}

func (l *namespacedLister) List(selector labels.Selector) ([]*apisv1beta2.ManagedCertificate, error) {
	var result []*apisv1beta2.ManagedCertificate

	for _, cert := range l.managedCertificates {
		if cert.Namespace == l.namespace {
			result = append(result, cert)
		}
	}

	return result, nil

}

func (l *namespacedLister) Get(name string) (*apisv1beta2.ManagedCertificate, error) {
	for _, cert := range l.managedCertificates {
		if cert.Namespace == l.namespace && cert.Name == name {
			return cert, nil
		}
	}

	return nil, errors.NewNotFound(groupResource, name)
}
