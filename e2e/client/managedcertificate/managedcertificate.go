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

// Package managedcertificate provides operations needed for interacting
// with ManagedCertificate resources in an e2e test.
package managedcertificate

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"

	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/networking.gke.io/v1"
	apisv1beta1 "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/networking.gke.io/v1beta1"
	apisv1beta2 "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/networking.gke.io/v1beta2"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/clientset/versioned"
	clientsetv1 "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/clientset/versioned/typed/networking.gke.io/v1"
	clientsetv1beta1 "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/clientset/versioned/typed/networking.gke.io/v1beta1"
	clientsetv1beta2 "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/clientset/versioned/typed/networking.gke.io/v1beta2"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/errors"
)

// Interface provides operations for interacting with ManagedCertificate
// resources in an e2e test.
type Interface interface {
	// CreateV1beta1 creates a ManagedCertificate v1beta1.
	CreateV1beta1(ctx context.Context, name string, domains []string) error
	// CreateV1beta2 creates a ManagedCertificate v1beta2.
	CreateV1beta2(ctx context.Context, name string, domains []string) error
	// Create creates a ManagedCertificate v1.
	Create(ctx context.Context, name string, domains []string) error
	// DeleteAll deletes all ManagedCertificates.
	DeleteAll(ctx context.Context) error
	// Delete deletes a ManagedCertificate.
	Delete(ctx context.Context, name string) error
	// Get fetches a ManagedCertificate.
	Get(ctx context.Context, name string) (*v1.ManagedCertificate, error)
	// Update updates a ManagedCertificate.
	Update(ctx context.Context, mcrt *v1.ManagedCertificate) error
}

type impl struct {
	// clientv1beta1 manages ManagedCertificate v1beta1 custom resources
	clientv1beta1 clientsetv1beta1.ManagedCertificateInterface
	// clientv1beta2 manages ManagedCertificate v1beta2 custom resources
	clientv1beta2 clientsetv1beta2.ManagedCertificateInterface
	// client manages ManagedCertificate custom resources
	client clientsetv1.ManagedCertificateInterface
}

func New(config *rest.Config, namespace string) (Interface, error) {
	clientset, err := versioned.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return impl{
		clientv1beta1: clientset.NetworkingV1beta1().ManagedCertificates(namespace),
		clientv1beta2: clientset.NetworkingV1beta2().ManagedCertificates(namespace),
		client:        clientset.NetworkingV1().ManagedCertificates(namespace),
	}, nil
}

func (m impl) CreateV1beta1(ctx context.Context, name string, domains []string) error {
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
	_, err := m.clientv1beta1.Create(ctx, mcrt, metav1.CreateOptions{})
	return err
}

func (m impl) CreateV1beta2(ctx context.Context, name string, domains []string) error {
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
	_, err := m.clientv1beta2.Create(ctx, mcrt, metav1.CreateOptions{})
	return err
}

func (m impl) Create(ctx context.Context, name string, domains []string) error {
	mcrt := &v1.ManagedCertificate{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1.ManagedCertificateSpec{
			Domains: domains,
		},
		Status: v1.ManagedCertificateStatus{
			DomainStatus: []v1.DomainStatus{},
		},
	}
	_, err := m.client.Create(ctx, mcrt, metav1.CreateOptions{})
	return err
}

func (m impl) DeleteAll(ctx context.Context) error {
	mcrts, err := m.client.List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, mcrt := range mcrts.Items {
		if err := m.Delete(ctx, mcrt.Name); err != nil {
			return err
		}
	}

	return nil
}

func (m impl) Delete(ctx context.Context, name string) error {
	return errors.IgnoreNotFound(m.client.Delete(ctx, name, metav1.DeleteOptions{}))
}

func (m impl) Get(ctx context.Context, name string) (*v1.ManagedCertificate, error) {
	return m.client.Get(ctx, name, metav1.GetOptions{})
}

func (m impl) Update(ctx context.Context, mcrt *v1.ManagedCertificate) error {
	_, err := m.client.Update(ctx, mcrt, metav1.UpdateOptions{})
	return err
}
