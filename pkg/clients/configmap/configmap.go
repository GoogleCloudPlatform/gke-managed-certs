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

// Package configmap provides operations for manipulating ConfigMap objects.
package configmap

import (
	api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"

	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/http"
)

// Interface exposes operations for manipulating ConfigMap resources.
type Interface interface {
	// Get fetches a ConfigMap.
	Get(namespace, name string) (*api.ConfigMap, error)
	// UpdateOrCreate updates or creates a ConfigMap.
	UpdateOrCreate(namespace string, configmap *api.ConfigMap) error
}

type impl struct {
	client v1.CoreV1Interface
}

func New(config *rest.Config) Interface {
	return impl{
		client: v1.NewForConfigOrDie(config),
	}
}

// Get fetches a ConfigMap.
func (c impl) Get(namespace, name string) (*api.ConfigMap, error) {
	return c.client.ConfigMaps(namespace).Get(name, metav1.GetOptions{})
}

// UpdateOrCreate updates or creates a ConfigMap.
func (c impl) UpdateOrCreate(namespace string, configmap *api.ConfigMap) error {
	configmaps := c.client.ConfigMaps(namespace)

	_, err := configmaps.Update(configmap)
	if !http.IsNotFound(err) {
		return err
	}

	_, err = configmaps.Create(configmap)
	return err
}
