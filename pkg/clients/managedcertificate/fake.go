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
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/util/workqueue"

	apisv1 "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/networking.gke.io/v1"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/types"
)

var (
	groupKind     = schema.GroupKind{Group: "networking.gke.io", Kind: "ManagedCertificate"}
	groupResource = schema.GroupResource{Group: "networking.gke.io", Resource: "ManagedCertificate"}
)

type Fake struct {
	managedCertificates []*apisv1.ManagedCertificate
}

var _ Interface = (*Fake)(nil)

func NewFake(managedCertificates []*apisv1.ManagedCertificate) *Fake {
	return &Fake{managedCertificates: managedCertificates}
}

func (f *Fake) Get(id types.CertId) (*apisv1.ManagedCertificate, error) {
	for _, cert := range f.managedCertificates {
		if cert.Namespace == id.Namespace && cert.Name == id.Name {
			return cert, nil
		}
	}

	return nil, errors.NewNotFound(groupResource, id.Name)
}

func (f *Fake) HasSynced() bool {
	return true
}

func (f *Fake) List() ([]*apisv1.ManagedCertificate, error) {
	return f.managedCertificates, nil
}

func (f *Fake) Update(managedCertificate *apisv1.ManagedCertificate) error {
	for i, cert := range f.managedCertificates {
		if cert.Namespace == managedCertificate.Namespace &&
			cert.Name == managedCertificate.Name {

			f.managedCertificates[i] = managedCertificate
			return nil
		}
	}

	return errors.NewNotFound(groupResource, managedCertificate.Name)
}

func (f *Fake) Run(ctx context.Context, queue workqueue.RateLimitingInterface) {}
