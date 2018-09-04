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

package controller

import (
	"sort"
	"testing"
)

var getPutDeleteTests = []struct {
	initArg   string
	initState SslCertificateState
	testArg   string
	outExists bool
	outState  SslCertificateState
	desc      string
}{
	{"", SslCertificateState{}, "cat", false, SslCertificateState{}, "Lookup argument in empty state"},
	{"cat", SslCertificateState{Current: "1", New: ""}, "cat", true, SslCertificateState{Current: "1", New: ""}, "Insert and lookup same argument, New empty"},
	{"tea", SslCertificateState{Current: "1", New: ""}, "dog", false, SslCertificateState{}, "Insert and lookup different arguments, New empty"},
	{"cat", SslCertificateState{Current: "1", New: "2"}, "cat", true, SslCertificateState{Current: "1", New: "2"}, "Insert and lookup same argument, New non-empty"},
	{"tea", SslCertificateState{Current: "1", New: "2"}, "dog", false, SslCertificateState{}, "Insert and lookup different arguments, New non-empty"},
}

func TestGetPutDelete(t *testing.T) {
	for _, testCase := range getPutDeleteTests {
		t.Run(testCase.desc, func(t *testing.T) {
			sut := newMcertState()

			if testCase.initArg != "" {
				sut.Put(testCase.initArg, testCase.initState)
			}

			if state, exists := sut.Get(testCase.testArg); exists != testCase.outExists {
				t.Errorf("Expected key %s to exist in state to be %t", testCase.testArg, testCase.outExists)
			} else {
				if state != testCase.outState {
					t.Errorf("Expected key %s to be mapped to %+v, instead is mapped to %+v", testCase.testArg, testCase.outState, state)
				}
			}

			if testCase.initArg != "" {
				sut.PutCurrent(testCase.initArg, testCase.initState.Current)
			}

			if state, exists := sut.Get(testCase.testArg); exists != testCase.outExists {
				t.Errorf("Expected key %s to exist in state to be %t", testCase.testArg, testCase.outExists)
			} else {
				if state.Current != testCase.outState.Current {
					t.Errorf("Expected key %s to be mapped to %s, instead is mapped to %s", testCase.testArg, testCase.outState.Current, state.Current)
				}

				if state.New != "" {
					t.Errorf("Expected New to be empty, instead is %s", state.New)
				}
			}

			sut.Delete("non existing key") // no-op

			sut.Delete(testCase.initArg)

			if state, exists := sut.Get(testCase.initArg); exists {
				t.Errorf("State should be empty after delete, instead is %+v", state)
			}
		})
	}
}

func eq(a, b []string) bool {
	sort.Strings(a)
	sort.Strings(b)

	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

func TestGetAll(t *testing.T) {
	state := newMcertState()

	state.PutCurrent("x", "1")
	state.Put("y", SslCertificateState{Current: "2", New: "3"})

	mcerts := state.GetAllManagedCertificates()
	if !eq(mcerts, []string{"x", "y"}) {
		t.Errorf("AllManagedCertificates expected to equal [x, y], instead are %v", mcerts)
	}

	sslCerts := state.GetAllSslCertificates()
	if !eq(sslCerts, []string{"1", "2", "3"}) {
		t.Errorf("AllSslCertificates expected to equal [1,2,3], instead are %v", sslCerts)
	}

	state.Delete("z") // no-op

	state.Delete("x")

	mcerts = state.GetAllManagedCertificates()
	if !eq(mcerts, []string{"y"}) {
		t.Errorf("AllManagedCertificates expected to equal [y], instead are %v", mcerts)
	}

	sslCerts = state.GetAllSslCertificates()
	if !eq(sslCerts, []string{"2", "3"}) {
		t.Errorf("AllSslCertificates expected to equal [2,3], instead are %v", sslCerts)
	}
}
