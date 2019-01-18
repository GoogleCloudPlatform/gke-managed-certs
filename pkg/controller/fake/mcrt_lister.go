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

package fake

import (
	"k8s.io/apimachinery/pkg/labels"

	api "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/networking.gke.io/v1beta1"
	mcrtlister "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/listers/networking.gke.io/v1beta1"
)

type fakeLister struct {
	managedCertificates []*api.ManagedCertificate
	err                 error
}

var _ mcrtlister.ManagedCertificateLister = &fakeLister{}

func NewLister(err error, managedCertificates []*api.ManagedCertificate) fakeLister {
	return fakeLister{
		managedCertificates: managedCertificates,
		err:                 err,
	}
}

func (f fakeLister) List(selector labels.Selector) ([]*api.ManagedCertificate, error) {
	return f.managedCertificates, f.err
}

func (f fakeLister) ManagedCertificates(namespace string) mcrtlister.ManagedCertificateNamespaceLister {
	return fakeNamespaceLister{
		managedCertificates: f.managedCertificates,
		err:                 f.err,
	}
}

type fakeNamespaceLister struct {
	managedCertificates []*api.ManagedCertificate
	err                 error
}

var _ mcrtlister.ManagedCertificateNamespaceLister = &fakeNamespaceLister{}

func (f fakeNamespaceLister) List(selector labels.Selector) ([]*api.ManagedCertificate, error) {
	return f.managedCertificates, f.err
}

func (f fakeNamespaceLister) Get(name string) (*api.ManagedCertificate, error) {
	for _, mcrt := range f.managedCertificates {
		if mcrt.Name == name {
			return mcrt, f.err
		}
	}

	return nil, f.err
}
