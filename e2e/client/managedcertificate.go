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

package client

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/gke.googleapis.com/v1alpha1"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/http"
)

func (m managedCertificate) Create(namespace, name string, domains []string) error {
	nsClient := m.clientset.GkeV1alpha1().ManagedCertificates(namespace)
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

func (m managedCertificate) DeleteAll(namespace string) error {
	mcrts, err := m.clientset.GkeV1alpha1().ManagedCertificates(namespace).List(metav1.ListOptions{})
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

func (m managedCertificate) Delete(namespace, name string) error {
	return http.IgnoreNotFound(m.clientset.GkeV1alpha1().ManagedCertificates(namespace).Delete(name, &metav1.DeleteOptions{}))
}

func (m managedCertificate) Get(namespace, name string) (*api.ManagedCertificate, error) {
	return m.clientset.GkeV1alpha1().ManagedCertificates(namespace).Get(name, metav1.GetOptions{})
}

func (m managedCertificate) Update(mcrt *api.ManagedCertificate) error {
	_, err := m.clientset.GkeV1alpha1().ManagedCertificates(mcrt.Namespace).Update(mcrt)
	return err
}
