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

package state

import (
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"

	api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/runtime"

	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/client/configmap"
)

type configMap interface {
	configmap.Client
	check(int)
}

// configMapMock counts the number of calls made to its methods.
type configMapMock struct {
	getCount    int
	changeCount int
	t           *testing.T
}

func (c *configMapMock) check(change int) {
	if c.getCount != 1 {
		c.t.Errorf("ConfigMap.Get() called %d times, want 1", c.getCount)
	}
	if c.changeCount != change {
		c.t.Errorf("ConfigMap.UpdateOrCreate() called %d times, want %d", c.changeCount, change)
	}
}

// failConfigMapMock fails Get and UpdateOrCreate with an error.
type failConfigMapMock struct {
	configMapMock
}

var _ configmap.Client = (*failConfigMapMock)(nil)

func (c *failConfigMapMock) Get(namespace, name string) (*api.ConfigMap, error) {
	c.getCount++
	return nil, errors.New("Fake error - failed to get a config map")
}

func (c *failConfigMapMock) UpdateOrCreate(namespace string, configmap *api.ConfigMap) error {
	c.changeCount++
	return errors.New("Fake error - failed to update or create a config map")
}

func newFail(t *testing.T) *failConfigMapMock {
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

var _ configmap.Client = (*emptyConfigMapMock)(nil)

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

var _ configmap.Client = (*filledConfigMapMock)(nil)

func (c *filledConfigMapMock) Get(namespace, name string) (*api.ConfigMap, error) {
	c.getCount++
	return &api.ConfigMap{Data: map[string]string{"1": "{\"Key\":\"default:cat\",\"Value\":\"1\"}"}}, nil
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

func deleteAndCheck(state *State, key Key, configmap configMap, changeCount *int) {
	state.Delete(key.Namespace, key.Name)
	(*changeCount)++
	configmap.check(*changeCount)
}

func putAndCheck(state *State, key Key, value string, configmap configMap, changeCount *int) {
	state.Put(key.Namespace, key.Name, value)
	(*changeCount)++
	configmap.check(*changeCount)
}

// Implementation of sorting for state Keys.
type keys []Key

func (k keys) Len() int {
	return len(k)
}

func (k keys) Swap(i, j int) {
	k[i], k[j] = k[j], k[i]
}

func (k keys) Less(i, j int) bool {
	ns := strings.Compare(k[i].Namespace, k[j].Namespace)
	return ns < 0 || (ns == 0 && strings.Compare(k[i].Name, k[j].Name) < 0)
}

func eq(a, b []Key) bool {
	sort.Sort(keys(a))
	sort.Sort(keys(b))

	return reflect.DeepEqual(a, b)
}

func TestState(t *testing.T) {
	testCases := []struct {
		configmap    configMap
		initKey      Key
		initVal      string
		testKey      Key
		outExists    bool
		outVal       string
		expectedKeys []Key
		desc         string
	}{
		{newFail(t), Key{"", ""}, "", Key{"default", "cat"}, false, "", nil, "Failing configmap - lookup argument in empty state"},
		{newFail(t), Key{"default", "cat"}, "1", Key{"default", "cat"}, true, "1", []Key{Key{"default", "cat"}}, "Failing configmap - insert and lookup same argument, same namespaces"},
		{newFail(t), Key{"default", "cat"}, "1", Key{"system", "cat"}, false, "", []Key{Key{"default", "cat"}}, "Failing configmap - insert and lookup same argument, different namespaces"},
		{newFail(t), Key{"default", "tea"}, "1", Key{"default", "dog"}, false, "", []Key{Key{"default", "tea"}}, "Failing configmap - insert and lookup different arguments, same namespace"},
		{newEmpty(t), Key{"", ""}, "", Key{"default", "cat"}, false, "", nil, "Empty configmap - lookup argument in empty state"},
		{newEmpty(t), Key{"default", "cat"}, "1", Key{"default", "cat"}, true, "1", []Key{Key{"default", "cat"}}, "Empty configmap - insert and lookup same argument, same namespaces"},
		{newEmpty(t), Key{"default", "cat"}, "1", Key{"system", "cat"}, false, "", []Key{Key{"default", "cat"}}, "Empty configmap - insert and lookup same argument, different namespaces"},
		{newEmpty(t), Key{"default", "tea"}, "1", Key{"default", "dog"}, false, "", []Key{Key{"default", "tea"}}, "Empty configmap - insert and lookup different arguments, same namespace"},
		{newFilled(t), Key{"", ""}, "", Key{"default", "cat"}, true, "1", []Key{Key{"default", "cat"}}, "Filled configmap - lookup argument in empty state"},
		{newFilled(t), Key{"default", "cat"}, "1", Key{"default", "cat"}, true, "1", []Key{Key{"default", "cat"}}, "Filled configmap - insert and lookup same argument, same namespaces"},
		{newFilled(t), Key{"default", "cat"}, "1", Key{"system", "cat"}, false, "", []Key{Key{"default", "cat"}}, "Filled configmap - insert and lookup same argument, different namespaces"},
		{newFilled(t), Key{"default", "tea"}, "1", Key{"default", "dog"}, false, "", []Key{Key{"default", "cat"}, Key{"default", "tea"}}, "Filled configmap - insert and lookup different arguments, same namespace"},
	}

	runtime.ErrorHandlers = nil

	for _, testCase := range testCases {
		t.Run(testCase.desc, func(t *testing.T) {
			changeCount := 0

			sut := New(testCase.configmap)
			testCase.configmap.check(changeCount)

			if testCase.initKey.Namespace != "" || testCase.initKey.Name != "" {
				putAndCheck(sut, testCase.initKey, testCase.initVal, testCase.configmap, &changeCount)
			}

			if value, exists := sut.Get(testCase.testKey.Namespace, testCase.testKey.Name); exists != testCase.outExists {
				t.Errorf("Expected key %+v to exist in state to be %t", testCase.testKey, testCase.outExists)
			} else if value != testCase.outVal {
				t.Errorf("%+v mapped to %s, want %s", testCase.testKey, value, testCase.outVal)
			}

			deleteAndCheck(sut, Key{"non existing namespace", "non existing key"}, testCase.configmap, &changeCount)

			foo := Key{"custom", "foo"}
			putAndCheck(sut, foo, "2", testCase.configmap, &changeCount)

			bar := Key{"custom", "bar"}
			putAndCheck(sut, bar, "3", testCase.configmap, &changeCount)

			mcrts := sut.GetAllKeys()
			expected := append(testCase.expectedKeys, foo, bar)
			if !eq(mcrts, expected) {
				t.Errorf("All ManagedCertificates are %v, want %v", mcrts, expected)
			}

			deleteAndCheck(sut, testCase.initKey, testCase.configmap, &changeCount)

			if value, exists := sut.Get(testCase.initKey.Namespace, testCase.initKey.Name); exists {
				t.Errorf("%+v mapped to %s after delete, want key missing", testCase.initKey, value)
			}
		})
	}
}
