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

package sync

import (
	compute "google.golang.org/api/compute/v0.beta"
	"k8s.io/client-go/discovery"
	fakediscovery "k8s.io/client-go/discovery/fake"
	cgo_testing "k8s.io/client-go/testing"

	api "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/gke.googleapis.com/v1alpha1"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/clientset/versioned"
	gkev1alpha1 "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/clientset/versioned/typed/gke.googleapis.com/v1alpha1"
	fakegkev1alpha1 "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/clientset/versioned/typed/gke.googleapis.com/v1alpha1/fake"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/errors"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/sslcertificatemanager"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/state"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/random"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/types"
)

const (
	channelBuffer = 10
	keySeparator  = ":"
	typeManaged   = "MANAGED"
)

// Fake ManagedCertificate clientset
type fakeClientset struct {
	cgo_testing.Fake
	discovery *fakediscovery.FakeDiscovery
}

var _ versioned.Interface = &fakeClientset{}

func newClientset() fakeClientset {
	f := fakeClientset{}
	f.discovery = &fakediscovery.FakeDiscovery{Fake: &f.Fake}
	return f
}

func (f fakeClientset) Discovery() discovery.DiscoveryInterface {
	return f.discovery
}

func (f fakeClientset) GkeV1alpha1() gkev1alpha1.GkeV1alpha1Interface {
	return &fakegkev1alpha1.FakeGkeV1alpha1{Fake: &f.Fake}
}

func (f fakeClientset) Gke() gkev1alpha1.GkeV1alpha1Interface {
	return &fakegkev1alpha1.FakeGkeV1alpha1{Fake: &f.Fake}
}

// Fake random
type fakeRandom struct {
	name string
	err  error
}

var _ random.Random = &fakeRandom{}

func newRandom(err error, name string) fakeRandom {
	return fakeRandom{
		name: name,
		err:  err,
	}
}

func (f fakeRandom) Name() (string, error) {
	return f.name, f.err
}

// Fake ssl manager
type fakeSsl struct {
	mapping   map[string]*compute.SslCertificate
	createErr error
	deleteErr error
	existsErr error
	getErr    error
}

var _ sslcertificatemanager.SslCertificateManager = &fakeSsl{}

func newSsl(key string, mcrt *api.ManagedCertificate, createErr, deleteErr, existsErr, getErr error) *fakeSsl {
	ssl := &fakeSsl{
		mapping: make(map[string]*compute.SslCertificate, 0),
	}

	if mcrt != nil {
		ssl.Create(key, *mcrt)
	}

	ssl.createErr = createErr
	ssl.deleteErr = deleteErr
	ssl.existsErr = existsErr
	ssl.getErr = getErr

	return ssl
}

func (f *fakeSsl) Create(sslCertificateName string, mcrt api.ManagedCertificate) error {
	f.mapping[sslCertificateName] = &compute.SslCertificate{
		Managed: &compute.SslCertificateManagedSslCertificate{
			Domains: mcrt.Spec.Domains,
		},
		Name: sslCertificateName,
		Type: typeManaged,
	}

	return f.createErr
}

func (f *fakeSsl) Delete(sslCertificateName string, mcrt *api.ManagedCertificate) error {
	delete(f.mapping, sslCertificateName)
	return f.deleteErr
}

func (f *fakeSsl) Exists(sslCertificateName string, mcrt *api.ManagedCertificate) (bool, error) {
	_, exists := f.mapping[sslCertificateName]
	return exists, f.existsErr
}

func (f *fakeSsl) Get(sslCertificateName string, mcrt *api.ManagedCertificate) (*compute.SslCertificate, error) {
	sslCert := f.mapping[sslCertificateName]
	return sslCert, f.getErr
}

// Fake state
type fakeMetricsState struct {
	softDeletedErr            error
	sslCertificateCreationErr error
}
type fakeState struct {
	entryExists                    bool
	id                             types.CertId
	sslCertificateName             string
	softDeleted                    bool
	sslCertificateCreationReported bool
	errOverride                    *fakeMetricsState
}

var _ state.State = &fakeState{}

func newEmptyState() *fakeState {
	return &fakeState{}
}

func newState(id types.CertId, sslCertificateName string) *fakeState {
	return &fakeState{
		id:                 id,
		sslCertificateName: sslCertificateName,
		entryExists:        true,
	}
}

func newStateWithOverride(id types.CertId, sslCertificateName string, sslCertificateCreationReported bool, sslCertificateCreationErr error,
	softDeleted bool, softDeletedErr error) *fakeState {

	state := newState(id, sslCertificateName)
	state.sslCertificateCreationReported = sslCertificateCreationReported
	state.softDeleted = softDeleted
	state.errOverride = &fakeMetricsState{
		softDeletedErr:            softDeletedErr,
		sslCertificateCreationErr: sslCertificateCreationErr,
	}
	return state
}

func (f *fakeState) Delete(id types.CertId) {
	f.entryExists = false
}

func (f *fakeState) ForeachKey(fun func(id types.CertId)) {
	fun(f.id)
}

func (f *fakeState) GetSslCertificateName(id types.CertId) (string, error) {
	if !f.entryExists {
		return "", errors.ErrManagedCertificateNotFound
	}

	return f.sslCertificateName, nil
}

func (f *fakeState) IsSoftDeleted(id types.CertId) (bool, error) {
	if f.errOverride != nil && f.errOverride.softDeletedErr != nil {
		return false, f.errOverride.softDeletedErr
	}

	if !f.entryExists {
		return false, errors.ErrManagedCertificateNotFound
	}

	return f.softDeleted, nil
}

func (f *fakeState) IsSslCertificateCreationReported(id types.CertId) (bool, error) {
	if f.errOverride != nil && f.errOverride.sslCertificateCreationErr != nil {
		return false, f.errOverride.sslCertificateCreationErr
	}

	if !f.entryExists {
		return false, errors.ErrManagedCertificateNotFound
	}

	return f.sslCertificateCreationReported, nil
}

func (f *fakeState) SetSoftDeleted(id types.CertId) error {
	if f.errOverride != nil && f.errOverride.softDeletedErr != nil {
		return f.errOverride.softDeletedErr
	}

	if !f.entryExists {
		return errors.ErrManagedCertificateNotFound
	}
	f.softDeleted = true

	return nil
}

func (f *fakeState) SetSslCertificateCreationReported(id types.CertId) error {
	if f.errOverride != nil && f.errOverride.sslCertificateCreationErr != nil {
		return f.errOverride.sslCertificateCreationErr
	}

	if !f.entryExists {
		return errors.ErrManagedCertificateNotFound
	}
	f.sslCertificateCreationReported = true

	return nil
}

func (f *fakeState) SetSslCertificateName(id types.CertId, sslCertificateName string) {
	f.sslCertificateName = sslCertificateName
	f.entryExists = true
}
