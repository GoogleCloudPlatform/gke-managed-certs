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
	"testing"

	"k8s.io/apimachinery/pkg/util/runtime"
)

func deleteEntry(state State, t tuple, configmap configMap, changeCount *int) {
	state.Delete(t.Namespace, t.Name)
	(*changeCount)++
	configmap.check(*changeCount)
}

func getSslCertificateName(t *testing.T, state State, tuple tuple, wantExists bool) {
	t.Helper()
	sslCertificateName, e := state.GetSslCertificateName(tuple.Namespace, tuple.Name)
	if e != wantExists {
		t.Fatalf("Expected %s:%s to exist in state to be %t", tuple.Namespace, tuple.Name, wantExists)
	}

	if e && sslCertificateName != tuple.SslCertificateName {
		t.Fatalf("%s:%s mapped to %s, want %s", tuple.Namespace, tuple.Name, sslCertificateName, tuple.SslCertificateName)
	}
}

func setSslCertificateCreationReported(state State, t tuple, configmap configMap, changeCount *int) {
	state.SetSslCertificateCreationReported(t.Namespace, t.Name)
	(*changeCount)++
	configmap.check(*changeCount)
}

func setSslCertificateName(state State, t tuple, configmap configMap, changeCount *int) {
	state.SetSslCertificateName(t.Namespace, t.Name, t.SslCertificateName)
	(*changeCount)++
	configmap.check(*changeCount)
}

func isSslCertificateCreationReported(t *testing.T, state State, tuple tuple, wantReported bool) {
	t.Helper()
	reported, e := state.IsSslCertificateCreationReported(tuple.Namespace, tuple.Name)
	if !e {
		t.Fatalf("Expected %s:%s to exist in state", tuple.Namespace, tuple.Name)
	} else if reported != wantReported {
		t.Fatalf("SslCertificate creation metric reported for %s:%s is %t, want %t", tuple.Namespace, tuple.Name, reported, wantReported)
	}
}

func contains(expected []tuple, namespace, name string) bool {
	for _, t := range expected {
		if t.Namespace == namespace && t.Name == name {
			return true
		}
	}

	return false
}

type tuple struct {
	Namespace          string
	Name               string
	SslCertificateName string
}

func (t tuple) empty() bool {
	return t.Namespace == "" && t.Name == ""
}

var tupleEmpty = tuple{"", "", ""}
var tupleDefaultCat1 = tuple{"default", "cat", "1"}
var tupleDefaultCatEmpty = tuple{"default", "cat", ""}
var tupleDefaultDogEmpty = tuple{"default", "dog", ""}
var tupleDefaultTea1 = tuple{"default", "tea", "1"}
var tupleSystemCatEmpty = tuple{"system", "cat", ""}

func TestState(t *testing.T) {
	testCases := []struct {
		desc      string
		configmap configMap
		init      tuple
		test      tuple
		exists    bool
		expected  []tuple
	}{
		{
			"Failing configmap - lookup argument in empty state",
			newFailing(t),
			tupleEmpty,
			tupleDefaultCatEmpty,
			false,
			nil,
		},
		{
			"Failing configmap - insert and lookup same argument, same namespaces",
			newFailing(t),
			tupleDefaultCat1,
			tupleDefaultCat1,
			true,
			[]tuple{tupleDefaultCat1},
		},
		{
			"Failing configmap - insert and lookup same argument, different namespaces",
			newFailing(t),
			tupleDefaultCat1,
			tupleSystemCatEmpty,
			false,
			[]tuple{tupleDefaultCat1},
		},
		{
			"Failing configmap - insert and lookup different arguments, same namespace",
			newFailing(t),
			tupleDefaultTea1,
			tupleDefaultDogEmpty,
			false,
			[]tuple{tupleDefaultTea1},
		},
		{
			"Empty configmap - lookup argument in empty state",
			newEmpty(t),
			tupleEmpty,
			tupleDefaultCatEmpty,
			false,
			nil,
		},
		{
			"Empty configmap - insert and lookup same argument, same namespaces",
			newEmpty(t),
			tupleDefaultCat1,
			tupleDefaultCat1,
			true,
			[]tuple{tupleDefaultCat1},
		},
		{
			"Empty configmap - insert and lookup same argument, different namespaces",
			newEmpty(t),
			tupleDefaultCat1,
			tupleSystemCatEmpty,
			false,
			[]tuple{tupleDefaultCat1},
		},
		{
			"Empty configmap - insert and lookup different arguments, same namespace",
			newEmpty(t),
			tupleDefaultTea1,
			tupleDefaultDogEmpty,
			false,
			[]tuple{tupleDefaultTea1},
		},
		{
			"Filled configmap - lookup argument in empty state",
			newFilled(t),
			tupleEmpty,
			tupleDefaultCat1,
			true,
			[]tuple{tupleDefaultCat1},
		},
		{
			"Filled configmap - insert and lookup same argument, same namespaces",
			newFilled(t),
			tupleDefaultCat1,
			tupleDefaultCat1,
			true,
			[]tuple{tupleDefaultCat1},
		},
		{
			"Filled configmap - insert and lookup same argument, different namespaces",
			newFilled(t),
			tupleDefaultCat1,
			tupleSystemCatEmpty,
			false,
			[]tuple{tupleDefaultCat1},
		},
		{
			"Filled configmap - insert and lookup different arguments, same namespace",
			newFilled(t),
			tupleDefaultTea1,
			tupleDefaultDogEmpty,
			false,
			[]tuple{tupleDefaultCat1, tupleDefaultTea1},
		},
	}

	runtime.ErrorHandlers = nil

	for _, testCase := range testCases {
		t.Run(testCase.desc, func(t *testing.T) {
			changeCount := 0

			sut := New(testCase.configmap)
			testCase.configmap.check(changeCount)

			if !testCase.init.empty() {
				setSslCertificateName(sut, testCase.init, testCase.configmap, &changeCount)
			}

			getSslCertificateName(t, sut, testCase.test, testCase.exists)

			deleteEntry(sut, tuple{"non existing namespace", "non existing name", ""}, testCase.configmap, &changeCount)

			foo := tuple{"custom", "foo", "2"}
			setSslCertificateName(sut, foo, testCase.configmap, &changeCount)
			isSslCertificateCreationReported(t, sut, foo, false)
			setSslCertificateCreationReported(sut, foo, testCase.configmap, &changeCount)
			isSslCertificateCreationReported(t, sut, foo, true)

			bar := tuple{"custom", "bar", "3"}
			setSslCertificateCreationReported(sut, bar, testCase.configmap, &changeCount)
			getSslCertificateName(t, sut, tuple{"custom", "bar", ""}, true)
			setSslCertificateName(sut, bar, testCase.configmap, &changeCount)

			expected := append(testCase.expected, foo, bar)
			allEntriesCounter := 0
			sut.ForeachKey(func(namespace, name string) {
				allEntriesCounter++
				if !contains(expected, namespace, name) {
					t.Fatalf("{%s, %s} missing in %v", namespace, name, expected)
				}
			})

			if allEntriesCounter != len(expected) {
				t.Fatalf("Found %d entries, want %d", allEntriesCounter, len(expected))
			}

			deleteEntry(sut, testCase.init, testCase.configmap, &changeCount)
			getSslCertificateName(t, sut, testCase.init, false)
		})
	}
}

func TestMarshal(t *testing.T) {
	m1 := map[string]entry{
		"mcrt1": entry{SslCertificateName: "sslCert1", SslCertificateCreationReported: false},
		"mcrt2": entry{SslCertificateName: "sslCert2", SslCertificateCreationReported: true},
	}
	m2 := unmarshal(marshal(m1))

	v, e := m2["mcrt1"]
	if !e || v.SslCertificateName != "sslCert1" || v.SslCertificateCreationReported != false {
		t.Fatalf("Marshalling and unmarshalling mangles data: e is %t, want true; v: %#v, want %#v", e, v, m1["mcrt1"])
	}

	v, e = m2["mcrt2"]
	if !e || v.SslCertificateName != "sslCert2" || v.SslCertificateCreationReported != true {
		t.Fatalf("Marshalling and unmarshalling mangles data: e is %t, want true; v: %#v, want %#v", e, v, m1["mcrt2"])
	}
}
