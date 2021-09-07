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

package ingress

import (
	"k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/config"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/types"
)

// IngressOption is an optional configuration parameter to Ingress.
type IngressOption func(*v1.Ingress)

// AnnotationManagedCertificates sets annotation "networking.gke.io/managed-certificates".
//
// Passing this option again will overwrite earlier values.
//
// Default: missing, Ingress.Annotations will have no key "networking.gke.io/managed-certificates"
func AnnotationManagedCertificates(value string) IngressOption {
	return func(ing *v1.Ingress) {
		if ing.Annotations == nil {
			ing.Annotations = make(map[string]string)
		}
		ing.Annotations[config.AnnotationManagedCertificatesKey] = value
	}
}

// AnnotationPreSharedCert sets annotation "ingress.gcp.kubernetes.io/pre-shared-cert".
//
// Passing this option again will overwrite earlier values.
//
// Default: missing, Ingress.Annotations will have no key "ingress.gcp.kubernetes.io/pre-shared-cert"
func AnnotationPreSharedCert(value string) IngressOption {
	return func(ing *v1.Ingress) {
		if ing.Annotations == nil {
			ing.Annotations = make(map[string]string)
		}
		ing.Annotations[config.AnnotationPreSharedCertKey] = value
	}
}

// New builds an Ingress for a given id and configures it using the given options.
func New(id types.Id, opts ...IngressOption) *v1.Ingress {
	ing := &v1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: id.Namespace,
			Name:      id.Name,
		},
	}
	for _, opt := range opts {
		opt(ing)
	}
	return ing
}
