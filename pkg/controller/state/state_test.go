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

	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/types"
)

type tuple struct {
	Id                 types.CertId
	SslCertificateName string
}

func (e tuple) empty() bool {
	return e.Id.Namespace == "" && e.Id.Name == ""
}

var (
	tupleEmpty           = tuple{types.NewCertId("", ""), ""}
	tupleDefaultCat1     = tuple{types.NewCertId("default", "cat"), "1"}
	tupleDefaultCatEmpty = tuple{types.NewCertId("default", "cat"), ""}
	tupleDefaultDogEmpty = tuple{types.NewCertId("default", "dog"), ""}
	tupleDefaultTea1     = tuple{types.NewCertId("default", "tea"), "1"}
	tupleSystemCatEmpty  = tuple{types.NewCertId("system", "cat"), ""}
)

func deleteEntry(state State, tuple tuple, configmap configMap, changeCount *int) {
	state.Delete(tuple.Id)
	(*changeCount)++
	configmap.check(*changeCount)
}

func getSslCertificateName(t *testing.T, state State, tuple tuple, wantExists bool) {
	t.Helper()
	sslCertificateName, err := state.GetSslCertificateName(tuple.Id)
	if err == nil != wantExists {
		t.Fatalf("Expected %s to exist in state to be %t", tuple.Id.String(), wantExists)
	} else if wantExists && sslCertificateName != tuple.SslCertificateName {
		t.Fatalf("%s mapped to %s, want %s", tuple.Id.String(), sslCertificateName, tuple.SslCertificateName)
	}
}

func isSoftDeleted(t *testing.T, state State, tuple tuple, wantSoftDeleted bool) {
	t.Helper()
	softDeleted, err := state.IsSoftDeleted(tuple.Id)
	if err != nil && wantSoftDeleted {
		t.Fatalf("Expected %s to exist in state, err: %s", tuple.Id.String(), err.Error())
	} else if softDeleted != wantSoftDeleted {
		t.Fatalf("Soft deleted for %s is %t, want %t", tuple.Id.String(), softDeleted, wantSoftDeleted)
	}
}

func isSslCertificateCreationReported(t *testing.T, state State, tuple tuple, wantReported bool) {
	t.Helper()
	reported, err := state.IsSslCertificateCreationReported(tuple.Id)
	if err != nil && wantReported {
		t.Fatalf("Expected %s to exist in state, err: %s", tuple.Id.String(), err.Error())
	} else if reported != wantReported {
		t.Fatalf("SslCertificate creation metric reported for %s is %t, want %t", tuple.Id.String(), reported, wantReported)
	}
}

func setSoftDeleted(state State, tuple tuple, configmap configMap, changeCount *int) error {
	if err := state.SetSoftDeleted(tuple.Id); err != nil {
		return err
	}

	(*changeCount)++
	configmap.check(*changeCount)
	return nil
}

func setSslCertificateCreationReported(state State, tuple tuple, configmap configMap, changeCount *int) error {
	if err := state.SetSslCertificateCreationReported(tuple.Id); err != nil {
		return err
	}

	(*changeCount)++
	configmap.check(*changeCount)
	return nil
}

func setSslCertificateName(state State, tuple tuple, configmap configMap, changeCount *int) {
	state.SetSslCertificateName(tuple.Id, tuple.SslCertificateName)
	(*changeCount)++
	configmap.check(*changeCount)
}

func contains(expected []tuple, id types.CertId) bool {
	for _, e := range expected {
		if e.Id.Namespace == id.Namespace && e.Id.Name == id.Name {
			return true
		}
	}

	return false
}

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

			deleteEntry(sut, tuple{types.NewCertId("non existing ns", "non existing name"), ""}, testCase.configmap, &changeCount)

			foo := tuple{types.NewCertId("custom", "foo"), "2"}
			isSslCertificateCreationReported(t, sut, foo, false)
			if err := setSslCertificateCreationReported(sut, foo, testCase.configmap, &changeCount); err == nil {
				t.Fatalf("Storing SslCertificate creation for non-existing entry should fail")
			}

			isSoftDeleted(t, sut, foo, false)
			if err := setSoftDeleted(sut, foo, testCase.configmap, &changeCount); err == nil {
				t.Fatalf("Setting soft deleted for non-existing entry should fail")
			}

			getSslCertificateName(t, sut, tuple{types.NewCertId("custom", "foo"), ""}, false)
			setSslCertificateName(sut, foo, testCase.configmap, &changeCount)

			isSslCertificateCreationReported(t, sut, foo, false)
			setSslCertificateCreationReported(sut, foo, testCase.configmap, &changeCount)
			isSslCertificateCreationReported(t, sut, foo, true)

			isSoftDeleted(t, sut, foo, false)
			setSoftDeleted(sut, foo, testCase.configmap, &changeCount)
			isSoftDeleted(t, sut, foo, true)

			expected := append(testCase.expected, foo)
			allEntriesCounter := 0
			sut.ForeachKey(func(id types.CertId) {
				allEntriesCounter++
				if !contains(expected, id) {
					t.Fatalf("%s missing in %v", id.String(), expected)
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
		"mcrt1": entry{
			SoftDeleted:                    false,
			SslCertificateName:             "sslCert1",
			SslCertificateCreationReported: false,
		},
		"mcrt2": entry{
			SoftDeleted:                    true,
			SslCertificateName:             "sslCert2",
			SslCertificateCreationReported: true,
		},
	}
	m2 := unmarshal(marshal(m1))

	v, e := m2["mcrt1"]
	if !e || v.SoftDeleted != false || v.SslCertificateName != "sslCert1" || v.SslCertificateCreationReported != false {
		t.Fatalf("Marshalling and unmarshalling mangles data: e is %t, want true; v: %#v, want %#v", e, v, m1["mcrt1"])
	}

	v, e = m2["mcrt2"]
	if !e || v.SoftDeleted != true || v.SslCertificateName != "sslCert2" || v.SslCertificateCreationReported != true {
		t.Fatalf("Marshalling and unmarshalling mangles data: e is %t, want true; v: %#v, want %#v", e, v, m1["mcrt2"])
	}
}
