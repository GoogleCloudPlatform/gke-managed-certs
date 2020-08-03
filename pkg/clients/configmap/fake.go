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

package configmap

import (
	"fmt"
	"strings"

	api "k8s.io/api/core/v1"

	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/errors"
)

const (
	separator = ":"
)

type fake struct {
	configmaps map[string]*api.ConfigMap
}

func NewFake() Interface {
	return &fake{configmaps: make(map[string]*api.ConfigMap, 0)}
}

func (f *fake) Get(namespace, name string) (*api.ConfigMap, error) {
	for k, v := range f.configmaps {
		pieces := strings.Split(k, separator)
		if len(pieces) != 2 {
			return nil, fmt.Errorf("invalid key %s", k)
		}

		if pieces[0] == namespace && pieces[1] == name {
			return v, nil
		}
	}

	return nil, errors.NotFound
}

func (f *fake) UpdateOrCreate(namespace string, configmap *api.ConfigMap) error {
	f.configmaps[fmt.Sprintf("%s%s%s", namespace, separator, configmap.Name)] = configmap
	return nil
}
