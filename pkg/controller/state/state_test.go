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
	"fmt"
	"reflect"
	"sort"
	"testing"

	api "k8s.io/api/core/v1"

	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/client/configmap"
)

type fakeConfigMock struct {
	getCount    int
	changeCount int
	t           *testing.T
}

var _ configmap.Client = (*fakeConfigMock)(nil)

func (f *fakeConfigMock) Get(namespace, name string) (*api.ConfigMap, error) {
	f.getCount++
	return nil, nil
}

func (f *fakeConfigMock) UpdateOrCreate(namespace string, configmap *api.ConfigMap) error {
	f.changeCount++
	return nil
}

func (f *fakeConfigMock) Check(change int) {
	if f.getCount != 1 {
		f.t.Errorf("ConfigMap.Get() expected to be called 1 times, was %d", f.getCount)
	}
	if f.changeCount != change {
		f.t.Errorf("ConfigMap.UpdateOrCreate() expected to be called %d times, was %d", change, f.changeCount)
	}
}

func deleteAndCheck(state *State, namespace, name string, configmap *fakeConfigMock, changeCount *int) {
	state.Delete(namespace, name)
	(*changeCount)++
	configmap.Check(*changeCount)
}

func putAndCheck(state *State, namespace, name, value string, configmap *fakeConfigMock, changeCount *int) {
	state.Put(namespace, name, value)
	(*changeCount)++
	configmap.Check(*changeCount)
}

func buildExpected(namespaces []string, names []string) []Key {
	var result []Key
	for i := range namespaces {
		if len(namespaces[i]) > 0 {
			result = append(result, Key{
				Namespace: namespaces[i],
				Name:      names[i],
			})
		}
	}

	return result
}

func eq(a, b []Key) bool {
	var x, y []string

	for i := range a {
		x = append(x, fmt.Sprintf("%s:%s", a[i].Namespace, a[i].Name))
		y = append(y, fmt.Sprintf("%s:%s", b[i].Namespace, b[i].Name))
	}

	sort.Strings(x)
	sort.Strings(y)

	return reflect.DeepEqual(x, y)
}

var getPutDeleteTests = []struct {
	initNamespace string
	initName      string
	initVal       string
	testNamespace string
	testName      string
	outExists     bool
	outVal        string
	desc          string
}{
	{"", "", "", "default", "cat", false, "", "Lookup argument in empty state"},
	{"default", "cat", "1", "default", "cat", true, "1", "Insert and lookup same argument, same namespaces"},
	{"default", "cat", "1", "system", "cat", false, "", "Insert and lookup same argument, different namespaces"},
	{"default", "tea", "1", "default", "dog", false, "", "Insert and lookup different arguments, same namespace"},
}

func TestGetPutDelete(t *testing.T) {
	for _, testCase := range getPutDeleteTests {
		t.Run(testCase.desc, func(t *testing.T) {
			changeCount := 0

			configmap := &fakeConfigMock{t: t}
			sut := New(configmap)
			configmap.Check(changeCount)

			if testCase.initNamespace != "" || testCase.initName != "" {
				putAndCheck(sut, testCase.initNamespace, testCase.initName, testCase.initVal, configmap, &changeCount)
			}

			if value, exists := sut.Get(testCase.testNamespace, testCase.testName); exists != testCase.outExists {
				t.Errorf("Expected key %s:%s to exist in state to be %t", testCase.testNamespace, testCase.testName, testCase.outExists)
			} else {
				if value != testCase.outVal {
					t.Errorf("Expected key %s:%s to be mapped to %s, instead is mapped to %s", testCase.testNamespace, testCase.testName, testCase.outVal, value)
				}
			}

			deleteAndCheck(sut, "non existing namespace", "non existing key", configmap, &changeCount)

			putAndCheck(sut, "default", "foo", "2", configmap, &changeCount)
			putAndCheck(sut, "default", "bar", "3", configmap, &changeCount)

			mcrts := sut.GetAllKeys()
			expected := buildExpected([]string{testCase.initNamespace, "default", "default"}, []string{testCase.initName, "foo", "bar"})
			if !eq(mcrts, expected) {
				t.Errorf("All ManagedCertificates expected to equal %v, instead are %v", expected, mcrts)
			}

			deleteAndCheck(sut, testCase.initNamespace, testCase.initName, configmap, &changeCount)

			if value, exists := sut.Get(testCase.initNamespace, testCase.initName); exists {
				t.Errorf("State should not contain %s:%s after delete, instead is %s", testCase.initNamespace, testCase.initName, value)
			}
		})
	}
}
