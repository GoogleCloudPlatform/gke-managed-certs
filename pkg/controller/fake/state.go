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

package fake

import (
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/errors"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/state"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/types"
)

type StateEntry struct {
	ExcludedFromSLO    bool
	ExcludedFromSLOErr error

	SoftDeleted    bool
	SoftDeletedErr error

	SslCertificateName string

	SslCertificateBindingReported bool
	SslCertificateBindingErr      error

	SslCertificateCreationReported bool
	SslCertificateCreationErr      error
}
type FakeState struct {
	data map[types.CertId]StateEntry
}

var _ state.State = &FakeState{}

func NewState() *FakeState {
	return &FakeState{
		data: make(map[types.CertId]StateEntry, 0),
	}
}

func NewStateWithEntries(data map[types.CertId]StateEntry) *FakeState {
	state := NewState()
	for k, v := range data {
		state.data[k] = v
	}
	return state
}

func (f *FakeState) Delete(id types.CertId) {
	delete(f.data, id)
}

func (f *FakeState) ForeachKey(fun func(id types.CertId)) {
	for k := range f.data {
		fun(k)
	}
}

func (f *FakeState) GetSslCertificateName(id types.CertId) (string, error) {
	entry, ok := f.data[id]
	if !ok {
		return "", errors.ErrManagedCertificateNotFound
	}

	return entry.SslCertificateName, nil
}

func (f *FakeState) IsExcludedFromSLO(id types.CertId) (bool, error) {
	entry, ok := f.data[id]
	if !ok {
		return false, errors.ErrManagedCertificateNotFound
	}
	if entry.ExcludedFromSLOErr != nil {
		return false, entry.ExcludedFromSLOErr
	}

	return entry.ExcludedFromSLO, nil
}

func (f *FakeState) IsSoftDeleted(id types.CertId) (bool, error) {
	entry, ok := f.data[id]
	if !ok {
		return false, errors.ErrManagedCertificateNotFound
	}
	if entry.SoftDeletedErr != nil {
		return false, entry.SoftDeletedErr
	}

	return entry.SoftDeleted, nil
}

func (f *FakeState) IsSslCertificateBindingReported(id types.CertId) (bool, error) {
	entry, ok := f.data[id]
	if !ok {
		return false, errors.ErrManagedCertificateNotFound
	}
	if entry.SslCertificateBindingErr != nil {
		return false, entry.SslCertificateBindingErr
	}

	return entry.SslCertificateBindingReported, nil
}

func (f *FakeState) IsSslCertificateCreationReported(id types.CertId) (bool, error) {
	entry, ok := f.data[id]
	if !ok {
		return false, errors.ErrManagedCertificateNotFound
	}
	if entry.SslCertificateCreationErr != nil {
		return false, entry.SslCertificateCreationErr
	}

	return entry.SslCertificateCreationReported, nil
}

func (f *FakeState) SetExcludedFromSLO(id types.CertId) error {
	entry, ok := f.data[id]
	if !ok {
		return errors.ErrManagedCertificateNotFound
	}
	if entry.ExcludedFromSLOErr != nil {
		return entry.ExcludedFromSLOErr
	}

	entry.ExcludedFromSLO = true
	f.data[id] = entry
	return nil
}

func (f *FakeState) SetSoftDeleted(id types.CertId) error {
	entry, ok := f.data[id]
	if !ok {
		return errors.ErrManagedCertificateNotFound
	}
	if entry.SoftDeletedErr != nil {
		return entry.SoftDeletedErr
	}

	entry.SoftDeleted = true
	f.data[id] = entry
	return nil
}

func (f *FakeState) SetSslCertificateBindingReported(id types.CertId) error {
	entry, ok := f.data[id]
	if !ok {
		return errors.ErrManagedCertificateNotFound
	}
	if entry.SslCertificateBindingErr != nil {
		return entry.SslCertificateBindingErr
	}

	entry.SslCertificateBindingReported = true
	f.data[id] = entry
	return nil
}

func (f *FakeState) SetSslCertificateCreationReported(id types.CertId) error {
	entry, ok := f.data[id]
	if !ok {
		return errors.ErrManagedCertificateNotFound
	}
	if entry.SslCertificateCreationErr != nil {
		return entry.SslCertificateCreationErr
	}

	entry.SslCertificateCreationReported = true
	f.data[id] = entry
	return nil
}

func (f *FakeState) SetSslCertificateName(id types.CertId, sslCertificateName string) {
	entry := f.data[id]
	entry.SslCertificateName = sslCertificateName
	f.data[id] = entry
}
