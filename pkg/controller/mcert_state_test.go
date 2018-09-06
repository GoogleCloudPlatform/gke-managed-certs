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
	initVal   string
	testArg   string
	outExists bool
	outVal    string
	desc      string
}{
	{"", "", "cat", false, "", "Lookup argument in empty state"},
	{"cat", "1", "cat", true, "1", "Insert and lookup same argument"},
	{"tea", "1", "dog", false, "", "Insert and lookup different arguments, New empty"},
}

func TestGetPutDelete(t *testing.T) {
	for _, testCase := range getPutDeleteTests {
		t.Run(testCase.desc, func(t *testing.T) {
			sut := newMcertState()

			if testCase.initArg != "" {
				sut.Put(testCase.initArg, testCase.initVal)
			}

			if value, exists := sut.Get(testCase.testArg); exists != testCase.outExists {
				t.Errorf("Expected key %s to exist in state to be %t", testCase.testArg, testCase.outExists)
			} else {
				if value != testCase.outVal {
					t.Errorf("Expected key %s to be mapped to %s, instead is mapped to %s", testCase.testArg, testCase.outVal, value)
				}
			}

			sut.Delete("non existing key") // no-op

			sut.Delete(testCase.initArg)

			if value, exists := sut.Get(testCase.initArg); exists {
				t.Errorf("State should be empty after delete, instead is %s", value)
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

	state.Put("x", "1")
	state.Put("y", "2")

	mcerts := state.GetAllManagedCertificates()
	if !eq(mcerts, []string{"x", "y"}) {
		t.Errorf("AllManagedCertificates expected to equal [x, y], instead are %v", mcerts)
	}

	sslCerts := state.GetAllSslCertificates()
	if !eq(sslCerts, []string{"1", "2"}) {
		t.Errorf("AllSslCertificates expected to equal [1,2], instead are %v", sslCerts)
	}

	state.Delete("z") // no-op

	state.Delete("x")

	mcerts = state.GetAllManagedCertificates()
	if !eq(mcerts, []string{"y"}) {
		t.Errorf("AllManagedCertificates expected to equal [y], instead are %v", mcerts)
	}

	sslCerts = state.GetAllSslCertificates()
	if !eq(sslCerts, []string{"2"}) {
		t.Errorf("AllSslCertificates expected to equal [2], instead are %v", sslCerts)
	}
}
