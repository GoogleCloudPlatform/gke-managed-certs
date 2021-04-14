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

// New builds an Ingress for a given id and annotations.
func New(id types.Id, annotationManagedCertificates,
	annotationPreSharedCert string) *v1.Ingress {

	return &v1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: id.Namespace,
			Name:      id.Name,
			Annotations: map[string]string{
				config.AnnotationManagedCertificatesKey: annotationManagedCertificates,
				config.AnnotationPreSharedCertKey:       annotationPreSharedCert,
			},
		},
	}
}
