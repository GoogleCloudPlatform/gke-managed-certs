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

// Package ingress provides helper functions for operating on Ingress resources.
package ingress

import (
	"k8s.io/api/networking/v1"
	"k8s.io/klog"

	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/config"
)

// IsGKE returns true if the parameter is a GKE Ingress.
func IsGKE(obj interface{}) bool {
	ingress := obj.(*v1.Ingress)

	if ingress == nil {
		klog.Errorf("invalid object type: %T", obj)
		return false
	}

	if ingress.Annotations == nil {
		return true // No annotations, treat as GKE Ingress.
	}

	ingressClass, ok := ingress.Annotations[config.AnnotationIngressClassKey]

	if !ok {
		return true // Lack of Ingress class annotation, treat as GKE Ingress.
	}

	if ingressClass == "" || ingressClass == "gce" {
		return true // Ingress class is empty or equals „gce”, treat as GKE Ingress.
	}

	return false // Unknown Ingress class
}
