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

func TestMcertState_GetPutDelete(t *testing.T) {
	state := newMcertState()

	_, e := state.Get("x")
	if e {
		t.Errorf("Fail, state should be empty")
		return
	}

	state.PutCurrent("x", "1")

	v, e := state.Get("x")
	if !e || v.Current != "1" {
		t.Errorf("Fail, x should be mapped to 1")
		return
	}

	state.PutCurrent("x", "2")

	v, e = state.Get("x")
	if !e || v.Current != "2" {
		t.Errorf("Fail, x should be mapped to 2")
	}

	state.Delete("y") // no-op

	state.Delete("x")

	_, e = state.Get("x")
	if e {
		t.Errorf("Fail, state should be empty after delete")
		return
	}
}

func TestMcertState_GetAllManagedCertificates_GetAllSslCertificates_Delete(t *testing.T) {
	state := newMcertState()

	state.PutCurrent("x", "1")
	state.PutState("y", SslCertificateState{Current: "2", New: "3"})

	mcerts := state.GetAllManagedCertificates()
	sort.Strings(mcerts)

	if mcerts[0] != "x" || mcerts[1] != "y" || len(mcerts) != 2 {
		t.Errorf("Fail, mcerts should have exactly two items x and y, instead %v", mcerts)
	}

	sslCerts := state.GetAllSslCertificates()
	sort.Strings(sslCerts)

	if sslCerts[0] != "1" || sslCerts[1] != "2" || sslCerts[2] != "3" || len(sslCerts) != 3 {
		t.Errorf("Fail, ssl certs should have exactly three items 1, 2 and 3, instead %v", sslCerts)
	}

	state.Delete("z") // no-op

	state.Delete("x")

	mcerts = state.GetAllManagedCertificates()
	if mcerts[0] != "y" || len(mcerts) != 1 {
		t.Errorf("Fail, mcerts should have exactly one item y, instead %v", mcerts)
	}

	sslCerts = state.GetAllSslCertificates()
	if sslCerts[0] != "2" || sslCerts[1] != "3" || len(sslCerts) != 2 {
		t.Errorf("Fail, ssl certs should have exactly two items 2 and 3, instead %v", sslCerts)
	}
}
