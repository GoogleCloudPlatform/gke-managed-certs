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

	apisv1 "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/networking.gke.io/v1"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/types"
)

type builder struct {
	managedCertificate *apisv1.ManagedCertificate
}

// New builds a ManagedCertificate for a given domain and id.
func New(id types.CertId, domain string) *builder {
	return &builder{
		&apisv1.ManagedCertificate{
			ObjectMeta: metav1.ObjectMeta{
				CreationTimestamp: metav1.Now().Rfc3339Copy(),
				Namespace:         id.Namespace,
				Name:              id.Name,
			},
			Spec: apisv1.ManagedCertificateSpec{
				Domains: []string{domain},
			},
			Status: apisv1.ManagedCertificateStatus{
				CertificateStatus: "",
				DomainStatus:      []apisv1.DomainStatus{},
			},
		},
	}
}

func (b *builder) WithStatus(status string, domainStatus ...string) *builder {
	b.managedCertificate.Status.CertificateStatus = status
	for i, domain := range b.managedCertificate.Spec.Domains {
		b.managedCertificate.Status.DomainStatus = append(b.managedCertificate.Status.DomainStatus, apisv1.DomainStatus{
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

func (b *builder) Build() *apisv1.ManagedCertificate {
	return b.managedCertificate
}
