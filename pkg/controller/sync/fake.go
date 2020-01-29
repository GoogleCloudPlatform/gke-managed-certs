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

package sync

import (
	"context"

	compute "google.golang.org/api/compute/v0.beta"

	apisv1beta2 "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/networking.gke.io/v1beta2"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/sslcertificatemanager"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/random"
)

const (
	channelBuffer = 10
	keySeparator  = ":"
	typeManaged   = "MANAGED"
)

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

func newSsl(key string, mcrt *apisv1beta2.ManagedCertificate, createErr, deleteErr, existsErr, getErr error) *fakeSsl {
	ssl := &fakeSsl{
		mapping: make(map[string]*compute.SslCertificate, 0),
	}

	if mcrt != nil {
		ssl.Create(context.Background(), key, *mcrt)
	}

	ssl.createErr = createErr
	ssl.deleteErr = deleteErr
	ssl.existsErr = existsErr
	ssl.getErr = getErr

	return ssl
}

func (f *fakeSsl) Create(ctx context.Context, sslCertificateName string, mcrt apisv1beta2.ManagedCertificate) error {
	f.mapping[sslCertificateName] = &compute.SslCertificate{
		Managed: &compute.SslCertificateManagedSslCertificate{
			Domains: mcrt.Spec.Domains,
		},
		Name: sslCertificateName,
		Type: typeManaged,
	}

	return f.createErr
}

func (f *fakeSsl) Delete(ctx context.Context, sslCertificateName string, mcrt *apisv1beta2.ManagedCertificate) error {
	delete(f.mapping, sslCertificateName)
	return f.deleteErr
}

func (f *fakeSsl) Exists(sslCertificateName string, mcrt *apisv1beta2.ManagedCertificate) (bool, error) {
	_, exists := f.mapping[sslCertificateName]
	return exists, f.existsErr
}

func (f *fakeSsl) Get(sslCertificateName string, mcrt *apisv1beta2.ManagedCertificate) (*compute.SslCertificate, error) {
	sslCert := f.mapping[sslCertificateName]
	return sslCert, f.getErr
}
