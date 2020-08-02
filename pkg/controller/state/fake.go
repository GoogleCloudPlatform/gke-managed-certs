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
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/errors"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/types"
)

type fake struct {
	mapping map[types.Id]Entry
}

var _ Interface = &fake{}

func NewFake() *fake {
	return &fake{mapping: make(map[types.Id]Entry, 0)}
}

func NewFakeWithEntries(data map[types.Id]Entry) Interface {
	state := NewFake()
	for k, v := range data {
		state.mapping[k] = v
	}
	return state
}

func (state *fake) Delete(id types.Id) {
	delete(state.mapping, id)
}

func (state *fake) Get(id types.Id) (Entry, error) {
	entry, exists := state.mapping[id]
	if !exists {
		return Entry{}, errors.NotFound
	}

	return entry, nil
}

func (state *fake) Insert(id types.Id, sslCertificateName string) {
	v, exists := state.mapping[id]
	if !exists {
		v = Entry{}
	}

	v.SslCertificateName = sslCertificateName

	state.mapping[id] = v
}

func (state *fake) List() map[types.Id]Entry {
	data := make(map[types.Id]Entry, 0)

	for id, entry := range state.mapping {
		data[id] = entry
	}

	return data
}

func (state *fake) SetExcludedFromSLO(id types.Id) error {
	v, exists := state.mapping[id]
	if !exists {
		return errors.NotFound
	}

	v.ExcludedFromSLO = true
	state.mapping[id] = v
	return nil
}

func (state *fake) SetSoftDeleted(id types.Id) error {
	v, exists := state.mapping[id]
	if !exists {
		return errors.NotFound
	}

	v.SoftDeleted = true
	state.mapping[id] = v
	return nil
}

func (state *fake) SetSslCertificateBindingReported(id types.Id) error {
	v, exists := state.mapping[id]
	if !exists {
		return errors.NotFound
	}

	v.SslCertificateBindingReported = true
	state.mapping[id] = v
	return nil
}

func (state *fake) SetSslCertificateCreationReported(id types.Id) error {
	v, exists := state.mapping[id]
	if !exists {
		return errors.NotFound
	}

	v.SslCertificateCreationReported = true
	state.mapping[id] = v
	return nil
}
