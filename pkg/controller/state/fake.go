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

package state

import (
	"errors"
	"testing"

	api "k8s.io/api/core/v1"

	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clients/configmap"
)

type configMap interface {
	configmap.ConfigMap
	check(int)
}

// configMapMock counts the number of calls made to its methods.
type configMapMock struct {
	getCount    int
	changeCount int
	t           *testing.T
}

func (c *configMapMock) check(change int) {
	c.t.Helper()

	if c.getCount != 1 {
		c.t.Fatalf("ConfigMap.Get() called %d times, want 1", c.getCount)
	}
	if c.changeCount != change {
		c.t.Fatalf("ConfigMap.UpdateOrCreate() called %d times, want %d", c.changeCount, change)
	}
}

// failConfigMapMock fails Get and UpdateOrCreate with an error.
type failConfigMapMock struct {
	configMapMock
}

var _ configmap.ConfigMap = (*failConfigMapMock)(nil)

func (c *failConfigMapMock) Get(namespace, name string) (*api.ConfigMap, error) {
	c.getCount++
	return nil, errors.New("Fake error - failed to get a config map")
}

func (c *failConfigMapMock) UpdateOrCreate(namespace string, configmap *api.ConfigMap) error {
	c.changeCount++
	return errors.New("Fake error - failed to update or create a config map")
}

func newFailing(t *testing.T) *failConfigMapMock {
	return &failConfigMapMock{
		configMapMock{
			t: t,
		},
	}
}

// emptyConfigMapMock represents a config map that is not initialized with any data.
type emptyConfigMapMock struct {
	configMapMock
}

var _ configmap.ConfigMap = (*emptyConfigMapMock)(nil)

func (c *emptyConfigMapMock) Get(namespace, name string) (*api.ConfigMap, error) {
	c.getCount++
	return &api.ConfigMap{Data: map[string]string{}}, nil
}

func (c *emptyConfigMapMock) UpdateOrCreate(namespace string, configmap *api.ConfigMap) error {
	c.changeCount++
	return nil
}

func newEmpty(t *testing.T) *emptyConfigMapMock {
	return &emptyConfigMapMock{
		configMapMock{
			t: t,
		},
	}
}

// filledConfigMapMock represents a config map that is initialized with data.
type filledConfigMapMock struct {
	configMapMock
}

var _ configmap.ConfigMap = (*filledConfigMapMock)(nil)

func (c *filledConfigMapMock) Get(namespace, name string) (*api.ConfigMap, error) {
	c.getCount++
	return &api.ConfigMap{
		Data: map[string]string{
			"1": "{\"Key\":{\"Namespace\":\"default\",\"Name\":\"cat\"},\"Value\":{\"SslCertificateName\":\"1\",\"SslCertificateCreationReported\":false}}",
		},
	}, nil
}

func (c *filledConfigMapMock) UpdateOrCreate(namespace string, configmap *api.ConfigMap) error {
	c.changeCount++
	return nil
}

func newFilled(t *testing.T) *filledConfigMapMock {
	return &filledConfigMapMock{
		configMapMock{
			t: t,
		},
	}
}
