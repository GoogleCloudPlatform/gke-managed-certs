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

	state.Put("x", "1")

	v, e := state.Get("x")
	if !e || v != "1" {
		t.Errorf("Fail, x should be mapped to 1")
		return
	}

	state.Put("x", "2")

	v, e = state.Get("x")
	if !e || v != "2" {
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

	state.Put("x", "1")
	state.Put("y", "2")

	mcerts := state.GetAllManagedCertificates()
	sort.Strings(mcerts)

	if mcerts[0] != "x" || mcerts[1] != "y" || len(mcerts) != 2 {
		t.Errorf("Fail, mcerts should have exactly two items x and y, instead %v", mcerts)
	}

	sslCerts := state.GetAllSslCertificates()
	sort.Strings(sslCerts)

	if sslCerts[0] != "1" || sslCerts[1] != "2" || len(sslCerts) != 2 {
		t.Errorf("Fail, ssl certs should have exactly two items 1 and 2, instead %v", sslCerts)
	}

	state.Delete("z") // no-op

	state.Delete("x")

	mcerts = state.GetAllManagedCertificates()
	if mcerts[0] != "y" || len(mcerts) != 1 {
		t.Errorf("Fail, mcerts should have exactly one item y, instead %v", mcerts)
	}

	sslCerts = state.GetAllSslCertificates()
	if sslCerts[0] != "2" || len(sslCerts) != 1 {
		t.Errorf("Fail, ssl certs should have exactly one item 2, instead %v", sslCerts)
	}
}
