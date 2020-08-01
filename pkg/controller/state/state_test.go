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

	"github.com/google/go-cmp/cmp"
	api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/runtime"

	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clients/configmap"
	cnterrors "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/errors"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/types"
)

type configMap interface {
	configmap.Interface
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

var _ configmap.Interface = (*failConfigMapMock)(nil)

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

var _ configmap.Interface = (*emptyConfigMapMock)(nil)

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

var _ configmap.Interface = (*filledConfigMapMock)(nil)

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

func TestState(t *testing.T) {
	runtime.ErrorHandlers = nil

	for description, testCase := range map[string]struct {
		configmap     configMap
		wantInitItems int
	}{
		"Failing configmap": {
			configmap:     newFailing(t),
			wantInitItems: 0,
		},
		"Empty configmap": {
			configmap:     newEmpty(t),
			wantInitItems: 0,
		},
		"Filled configmap": {
			configmap:     newFilled(t),
			wantInitItems: 1,
		},
	} {
		t.Run(description, func(t *testing.T) {
			// Create a state instance.
			state := New(testCase.configmap)

			// At the beginning there are wantInitItems items in state.
			if len(state.List()) != testCase.wantInitItems {
				t.Fatalf("len(List()) = %d, want %d", len(state.List()), testCase.wantInitItems)
			}

			changeCount := 0
			testCase.configmap.check(changeCount)

			// Getting a key not present in state fails.
			missingId := types.NewCertId("default", "missing")
			entry, err := state.Get(missingId)
			if err != cnterrors.ErrManagedCertificateNotFound {
				t.Fatalf("Get(%s): %v, want %v",
					missingId.String(), err, cnterrors.ErrManagedCertificateNotFound)
			}

			// Setting flags on a missing item fails.
			if err := state.SetExcludedFromSLO(missingId); err != cnterrors.ErrManagedCertificateNotFound {
				t.Fatalf("SetExcludedFromSLO(%s): %v, want %v",
					missingId.String(), err, cnterrors.ErrManagedCertificateNotFound)
			}
			testCase.configmap.check(changeCount)

			if err := state.SetSoftDeleted(missingId); err != cnterrors.ErrManagedCertificateNotFound {
				t.Fatalf("SetSoftDeleted(%s): %v, want %v",
					missingId.String(), err, cnterrors.ErrManagedCertificateNotFound)
			}
			testCase.configmap.check(changeCount)

			if err := state.SetSslCertificateBindingReported(missingId); err != cnterrors.ErrManagedCertificateNotFound {
				t.Fatalf("SetSslCertificateBindingReported(%s): %v, want %v",
					missingId.String(), err, cnterrors.ErrManagedCertificateNotFound)
			}
			testCase.configmap.check(changeCount)

			if err := state.SetSslCertificateCreationReported(missingId); err != cnterrors.ErrManagedCertificateNotFound {
				t.Fatalf("SetSslCertificateCreationReported(%s): %v, want %v",
					missingId.String(), err, cnterrors.ErrManagedCertificateNotFound)
			}
			testCase.configmap.check(changeCount)

			// Add an item to state.
			id := types.NewCertId("default", "foo")
			state.Insert(id, "foo")
			changeCount++
			testCase.configmap.check(changeCount)

			// The new item can be retrieved.
			entry, err = state.Get(id)
			if err != nil {
				t.Fatalf("Get(%s): %v, want nil", id.String(), err)
			}
			if diff := cmp.Diff(Entry{SslCertificateName: "foo"}, entry); diff != "" {
				t.Fatalf("Get(%s): (-want, +got): %s", id.String(), diff)
			}

			// There are in total wantInitItems+1 entries in state.
			if len(state.List()) != testCase.wantInitItems+1 {
				t.Fatalf("len(List()) = %d, want %d", len(state.List()), testCase.wantInitItems+1)
			}

			// Set all the flags one by one.
			if err := state.SetExcludedFromSLO(id); err != nil {
				t.Fatalf("SetExcludedFromSLO(%s): %v, want nil", id.String(), err)
			}
			changeCount++
			testCase.configmap.check(changeCount)

			if err := state.SetSoftDeleted(id); err != nil {
				t.Fatalf("SetSoftDeleted(%s): %v, want nil", id.String(), err)
			}
			changeCount++
			testCase.configmap.check(changeCount)

			if err := state.SetSslCertificateBindingReported(id); err != nil {
				t.Fatalf("SetSslCertificateBindingReported(%s): %v, want nil", id.String(), err)
			}
			changeCount++
			testCase.configmap.check(changeCount)

			if err := state.SetSslCertificateCreationReported(id); err != nil {
				t.Fatalf("SetSslCertificateCreationReported(%s): %v, want nil", id.String(), err)
			}
			changeCount++
			testCase.configmap.check(changeCount)

			// Delete the item
			state.Delete(id)
			changeCount++
			testCase.configmap.check(changeCount)

			// There are wantInitItems items in state.
			if len(state.List()) != testCase.wantInitItems {
				t.Fatalf("len(List()) = %d, want %d", len(state.List()), testCase.wantInitItems)
			}
		})
	}
}

func TestMarshal(t *testing.T) {
	mcrt1 := types.NewCertId("default", "mcrt1")
	mcrt2 := types.NewCertId("system", "mcrt2")

	m1 := map[types.CertId]Entry{
		mcrt1: Entry{
			SoftDeleted:                    false,
			SslCertificateName:             "sslCert1",
			SslCertificateCreationReported: false,
		},
		mcrt2: Entry{
			SoftDeleted:                    true,
			SslCertificateName:             "sslCert2",
			SslCertificateCreationReported: true,
		},
	}
	m2 := unmarshal(marshal(m1))

	v, e := m2[mcrt1]
	if !e || v.SoftDeleted != false || v.SslCertificateName != "sslCert1" || v.SslCertificateCreationReported != false {
		t.Fatalf("Marshalling and unmarshalling mangles data: e is %t, want true; v: %#v, want %#v", e, v, m1[mcrt1])
	}

	v, e = m2[mcrt2]
	if !e || v.SoftDeleted != true || v.SslCertificateName != "sslCert2" || v.SslCertificateCreationReported != true {
		t.Fatalf("Marshalling and unmarshalling mangles data: e is %t, want true; v: %#v, want %#v", e, v, m1[mcrt2])
	}
}
