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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	api "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/networking.gke.io/v1beta1"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/types"
)

// NewManagedCertificate builds an active ManagedCertificate for a given domain and id.
func NewManagedCertificate(id types.CertId, domain string) *api.ManagedCertificate {
	return &api.ManagedCertificate{
		ObjectMeta: metav1.ObjectMeta{
			CreationTimestamp: metav1.Now().Rfc3339Copy(),
			Namespace:         id.Namespace,
			Name:              id.Name,
		},
		Spec: api.ManagedCertificateSpec{
			Domains: []string{domain},
		},
		Status: api.ManagedCertificateStatus{
			CertificateStatus: "Active",
		},
	}
}
