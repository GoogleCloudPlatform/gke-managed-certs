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
	"testing"

	api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/runtime"

	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/client/configmap"
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

var _ configmap.ConfigMap = (*failConfigMapMock)(nil)

func (c *failConfigMapMock) Get(namespace, name string) (*api.ConfigMap, error) {
	c.getCount++
	return nil, errors.New("Fake error - failed to get a config map")
}

func (c *failConfigMapMock) UpdateOrCreate(namespace string, configmap *api.ConfigMap) error {
	c.changeCount++
	return errors.New("Fake error - failed to update or create a config map")
}

func newFails(t *testing.T) *failConfigMapMock {
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

func deleteAndCheck(state *State, t tuple, configmap configMap, changeCount *int) {
	state.Delete(t.Namespace, t.Name)
	(*changeCount)++
	configmap.check(*changeCount)
}

func putAndCheck(state *State, t tuple, configmap configMap, changeCount *int) {
	state.Put(t.Namespace, t.Name, t.Value)
	(*changeCount)++
	configmap.check(*changeCount)
}

func contains(expected []tuple, namespace, name, value string) bool {
	for _, t := range expected {
		if t.Namespace == namespace && t.Name == name && t.Value == value {
			return true
		}
	}

	return false
}

type tuple struct {
	Namespace string
	Name      string
	Value     string
}

func (t tuple) empty() bool {
	return t.Namespace == "" && t.Name == ""
}

var emptytp = tuple{"", "", ""}
var defcat1 = tuple{"default", "cat", "1"}
var defcate = tuple{"default", "cat", ""}
var defdoge = tuple{"default", "dog", ""}
var syscate = tuple{"system", "cat", ""}
var deftea1 = tuple{"default", "tea", "1"}

func TestState(t *testing.T) {
	testCases := []struct {
		configmap configMap
		init      tuple
		test      tuple
		exists    bool
		expected  []tuple
		desc      string
	}{
		{newFails(t), emptytp, defcate, false, nil, "Failing configmap - lookup argument in empty state"},
		{newFails(t), defcat1, defcat1, true, []tuple{defcat1}, "Failing configmap - insert and lookup same argument, same namespaces"},
		{newFails(t), defcat1, syscate, false, []tuple{defcat1}, "Failing configmap - insert and lookup same argument, different namespaces"},
		{newFails(t), deftea1, defdoge, false, []tuple{deftea1}, "Failing configmap - insert and lookup different arguments, same namespace"},
		{newEmpty(t), emptytp, defcate, false, nil, "Empty configmap - lookup argument in empty state"},
		{newEmpty(t), defcat1, defcat1, true, []tuple{defcat1}, "Empty configmap - insert and lookup same argument, same namespaces"},
		{newEmpty(t), defcat1, syscate, false, []tuple{defcat1}, "Empty configmap - insert and lookup same argument, different namespaces"},
		{newEmpty(t), deftea1, defdoge, false, []tuple{deftea1}, "Empty configmap - insert and lookup different arguments, same namespace"},
		{newFilled(t), emptytp, defcat1, true, []tuple{defcat1}, "Filled configmap - lookup argument in empty state"},
		{newFilled(t), defcat1, defcat1, true, []tuple{defcat1}, "Filled configmap - insert and lookup same argument, same namespaces"},
		{newFilled(t), defcat1, syscate, false, []tuple{defcat1}, "Filled configmap - insert and lookup same argument, different namespaces"},
		{newFilled(t), deftea1, defdoge, false, []tuple{defcat1, deftea1}, "Filled configmap - insert and lookup different arguments, same namespace"},
	}

	runtime.ErrorHandlers = nil

	for _, testCase := range testCases {
		t.Run(testCase.desc, func(t *testing.T) {
			changeCount := 0

			sut := New(testCase.configmap)
			testCase.configmap.check(changeCount)

			if !testCase.init.empty() {
				putAndCheck(sut, testCase.init, testCase.configmap, &changeCount)
			}

			if value, exists := sut.Get(testCase.test.Namespace, testCase.test.Name); exists != testCase.exists {
				t.Errorf("Expected %s:%s to exist in state to be %t", testCase.test.Namespace, testCase.test.Name, testCase.exists)
			} else if value != testCase.test.Value {
				t.Errorf("%s:%s mapped to %s, want %s", testCase.test.Namespace, testCase.test.Name, value, testCase.test.Value)
			}

			deleteAndCheck(sut, tuple{"non existing namespace", "non existing name", ""}, testCase.configmap, &changeCount)

			foo := tuple{"custom", "foo", "2"}
			putAndCheck(sut, foo, testCase.configmap, &changeCount)

			bar := tuple{"custom", "bar", "3"}
			putAndCheck(sut, bar, testCase.configmap, &changeCount)

			expected := append(testCase.expected, foo, bar)
			allEntriesCounter := 0
			sut.Foreach(func(namespace, name, value string) {
				allEntriesCounter++
				if !contains(expected, namespace, name, value) {
					t.Errorf("{%s, %s, %s} missing in %v", namespace, name, value, expected)
				}
			})

			if allEntriesCounter != len(expected) {
				t.Errorf("Found %d entries, want %d", allEntriesCounter, len(expected))
			}

			deleteAndCheck(sut, testCase.init, testCase.configmap, &changeCount)

			if value, exists := sut.Get(testCase.init.Namespace, testCase.init.Name); exists {
				t.Errorf("%s:%s mapped to %s after delete, want key missing", testCase.init.Namespace, testCase.init.Name, value)
			}
		})
	}
}
