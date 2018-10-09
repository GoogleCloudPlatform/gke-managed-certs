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
	"reflect"
	"sort"
	"strings"
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

func deleteAndCheck(state *State, key Key, configmap *fakeConfigMock, changeCount *int) {
	state.Delete(key.Namespace, key.Name)
	(*changeCount)++
	configmap.Check(*changeCount)
}

func putAndCheck(state *State, key Key, value string, configmap *fakeConfigMock, changeCount *int) {
	state.Put(key.Namespace, key.Name, value)
	(*changeCount)++
	configmap.Check(*changeCount)
}

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

func TestGetPutDelete(t *testing.T) {
	testCases := []struct {
		initKey   Key
		initVal   string
		testKey   Key
		outExists bool
		outVal    string
		desc      string
	}{
		{Key{"", ""}, "", Key{"default", "cat"}, false, "", "Lookup argument in empty state"},
		{Key{"default", "cat"}, "1", Key{"default", "cat"}, true, "1", "Insert and lookup same argument, same namespaces"},
		{Key{"default", "cat"}, "1", Key{"system", "cat"}, false, "", "Insert and lookup same argument, different namespaces"},
		{Key{"default", "tea"}, "1", Key{"default", "dog"}, false, "", "Insert and lookup different arguments, same namespace"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.desc, func(t *testing.T) {
			changeCount := 0

			configmap := &fakeConfigMock{t: t}
			sut := New(configmap)
			configmap.Check(changeCount)

			if testCase.initKey.Namespace != "" || testCase.initKey.Name != "" {
				putAndCheck(sut, testCase.initKey, testCase.initVal, configmap, &changeCount)
			}

			if value, exists := sut.Get(testCase.testKey.Namespace, testCase.testKey.Name); exists != testCase.outExists {
				t.Errorf("Expected key %+v to exist in state to be %t", testCase.testKey, testCase.outExists)
			} else if value != testCase.outVal {
				t.Errorf("%+v mapped to %s, want %s", testCase.testKey, value, testCase.outVal)
			}

			deleteAndCheck(sut, Key{"non existing namespace", "non existing key"}, configmap, &changeCount)

			foo := Key{"custom", "foo"}
			putAndCheck(sut, foo, "2", configmap, &changeCount)

			bar := Key{"custom", "bar"}
			putAndCheck(sut, bar, "3", configmap, &changeCount)

			mcrts := sut.GetAllKeys()
			expected := []Key{foo, bar}
			if testCase.initKey.Namespace != "" {
				expected = append(expected, testCase.initKey)
			}
			if !eq(mcrts, expected) {
				t.Errorf("All ManagedCertificates are %v, want %v", mcrts, expected)
			}

			deleteAndCheck(sut, testCase.initKey, configmap, &changeCount)

			if value, exists := sut.Get(testCase.initKey.Namespace, testCase.initKey.Name); exists {
				t.Errorf("%+v mapped to %s after delete, want key missing", testCase.initKey, value)
			}
		})
	}
}
