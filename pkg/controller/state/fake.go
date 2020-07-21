/*
Copyright 2020 Google LLC

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
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/errors"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/types"
)

type fakeState struct {
	mapping map[types.CertId]Entry
}

var _ State = &fakeState{}

func NewFake() *fakeState {
	return &fakeState{
		mapping: make(map[types.CertId]Entry, 0),
	}
}

func NewFakeWithEntries(data map[types.CertId]Entry) State {
	state := NewFake()
	for k, v := range data {
		state.mapping[k] = v
	}
	return state
}

func (state *fakeState) Delete(id types.CertId) {
	delete(state.mapping, id)
}

func (state *fakeState) Get(id types.CertId) (Entry, error) {
	entry, exists := state.mapping[id]
	if !exists {
		return Entry{}, errors.ErrManagedCertificateNotFound
	}

	return entry, nil
}

func (state *fakeState) Insert(id types.CertId, sslCertificateName string) {
	v, exists := state.mapping[id]
	if !exists {
		v = Entry{}
	}

	v.SslCertificateName = sslCertificateName

	state.mapping[id] = v
}

func (state *fakeState) List() map[types.CertId]Entry {
	data := make(map[types.CertId]Entry, 0)

	for id, entry := range state.mapping {
		data[id] = entry
	}

	return data
}

func (state *fakeState) SetExcludedFromSLO(id types.CertId) error {
	v, exists := state.mapping[id]
	if !exists {
		return errors.ErrManagedCertificateNotFound
	}

	v.ExcludedFromSLO = true
	state.mapping[id] = v
	return nil
}

func (state *fakeState) SetSoftDeleted(id types.CertId) error {
	v, exists := state.mapping[id]
	if !exists {
		return errors.ErrManagedCertificateNotFound
	}

	v.SoftDeleted = true
	state.mapping[id] = v
	return nil
}

func (state *fakeState) SetSslCertificateBindingReported(id types.CertId) error {
	v, exists := state.mapping[id]
	if !exists {
		return errors.ErrManagedCertificateNotFound
	}

	v.SslCertificateBindingReported = true
	state.mapping[id] = v
	return nil
}

func (state *fakeState) SetSslCertificateCreationReported(id types.CertId) error {
	v, exists := state.mapping[id]
	if !exists {
		return errors.ErrManagedCertificateNotFound
	}

	v.SslCertificateCreationReported = true
	state.mapping[id] = v
	return nil
}
