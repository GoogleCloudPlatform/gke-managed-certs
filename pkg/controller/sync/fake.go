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
	"errors"
	"fmt"
	"strings"

	compute "google.golang.org/api/compute/v0.beta"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/discovery"
	fakediscovery "k8s.io/client-go/discovery/fake"
	cgo_testing "k8s.io/client-go/testing"

	api "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/gke.googleapis.com/v1alpha1"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/clientset/versioned"
	gkev1alpha1 "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/clientset/versioned/typed/gke.googleapis.com/v1alpha1"
	fakegkev1alpha1 "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/clientset/versioned/typed/gke.googleapis.com/v1alpha1/fake"
	mcrtlister "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/listers/gke.googleapis.com/v1alpha1"
)

const (
	channelBuffer = 10
	keySeparator  = ":"
	typeManaged   = "MANAGED"
)

// Fake lister
type fakeLister struct {
	managedCertificate *api.ManagedCertificate
	err                error
}

func newLister(err error, managedCertificate *api.ManagedCertificate) fakeLister {
	return fakeLister{
		managedCertificate: managedCertificate,
		err:                err,
	}
}

func (f fakeLister) List(selector labels.Selector) ([]*api.ManagedCertificate, error) {
	return nil, errors.New("Not implemented")
}

func (f fakeLister) ManagedCertificates(namespace string) mcrtlister.ManagedCertificateNamespaceLister {
	return fakeNamespaceLister{
		managedCertificate: f.managedCertificate,
		err:                f.err,
	}
}

type fakeNamespaceLister struct {
	managedCertificate *api.ManagedCertificate
	err                error
}

func (f fakeNamespaceLister) List(selector labels.Selector) ([]*api.ManagedCertificate, error) {
	return nil, errors.New("Not implemented")
}

func (f fakeNamespaceLister) Get(name string) (*api.ManagedCertificate, error) {
	return f.managedCertificate, f.err
}

// Fake ManagedCertificate clientset
type fakeClientset struct {
	cgo_testing.Fake
	discovery *fakediscovery.FakeDiscovery
}

func newClientset() fakeClientset {
	f := fakeClientset{}
	f.discovery = &fakediscovery.FakeDiscovery{Fake: &f.Fake}
	return f
}

func (f fakeClientset) Discovery() discovery.DiscoveryInterface {
	return f.discovery
}

var _ versioned.Interface = &fakeClientset{}

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

func newRandom(err error, name string) fakeRandom {
	return fakeRandom{
		name: name,
		err:  err,
	}
}

func (f fakeRandom) Name() (string, error) {
	return f.name, f.err
}

// Fake state
type fakeState struct {
	mapping map[string]string
}

func buildKey(namespace, name string) string {
	return fmt.Sprintf("%s%s%s", namespace, keySeparator, name)
}

func splitKey(key string) (string, string) {
	parts := strings.Split(key, keySeparator)
	return parts[0], parts[1]
}

func newState(namespace, name, value string) *fakeState {
	state := &fakeState{
		mapping: make(map[string]string, 0),
	}

	if namespace != "" {
		state.Put(namespace, name, value)
	}

	return state
}

func (f *fakeState) Delete(namespace, name string) {
	delete(f.mapping, buildKey(namespace, name))
}

func (f *fakeState) ForeachKey(fun func(namespace, name string)) {
	for k := range f.mapping {
		namespace, name := splitKey(k)
		fun(namespace, name)
	}
}

func (f *fakeState) Get(namespace, name string) (string, bool) {
	v, e := f.mapping[buildKey(namespace, name)]
	return v, e
}

func (f *fakeState) Put(namespace, name, value string) {
	f.mapping[buildKey(namespace, name)] = value
}

// Fake ssl manager
type fakeSsl struct {
	mapping   map[string]*compute.SslCertificate
	createErr <-chan error
	deleteErr <-chan error
	existsErr <-chan error
	getErr    <-chan error
}

func newSsl(key string, mcrt *api.ManagedCertificate, createErr, deleteErr, existsErr, getErr []error) *fakeSsl {
	ssl := &fakeSsl{
		mapping: make(map[string]*compute.SslCertificate, 0),
	}

	if mcrt != nil {
		ssl.Create(key, *mcrt)
	}

	ssl.createErr = toChannel(createErr)
	ssl.deleteErr = toChannel(deleteErr)
	ssl.existsErr = toChannel(existsErr)
	ssl.getErr = toChannel(getErr)

	return ssl
}

func toChannel(err []error) <-chan error {
	ch := make(chan error, channelBuffer)
	for _, e := range err {
		ch <- e
	}

	return ch
}

func errOrNil(ch <-chan error) error {
	select {
	case e := <-ch:
		return e
	default:
		return nil
	}
}

func (f *fakeSsl) Create(sslCertificateName string, mcrt api.ManagedCertificate) error {
	f.mapping[sslCertificateName] = &compute.SslCertificate{
		Managed: &compute.SslCertificateManagedSslCertificate{
			Domains: mcrt.Spec.Domains,
		},
		Name: sslCertificateName,
		Type: typeManaged,
	}

	return errOrNil(f.createErr)
}

func (f *fakeSsl) Delete(sslCertificateName string, mcrt *api.ManagedCertificate) error {
	delete(f.mapping, sslCertificateName)
	return errOrNil(f.deleteErr)
}

func (f *fakeSsl) Exists(sslCertificateName string, mcrt *api.ManagedCertificate) (bool, error) {
	_, exists := f.mapping[sslCertificateName]
	return exists, errOrNil(f.existsErr)
}

func (f *fakeSsl) Get(sslCertificateName string, mcrt *api.ManagedCertificate) (*compute.SslCertificate, error) {
	sslCert := f.mapping[sslCertificateName]
	return sslCert, errOrNil(f.getErr)
}
