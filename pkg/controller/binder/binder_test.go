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

package binder

import (
	"errors"
	"reflect"
	"sort"
	"testing"

	api "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes/typed/extensions/v1beta1/fake"
	lister "k8s.io/client-go/listers/extensions/v1beta1"
	cgo_testing "k8s.io/client-go/testing"

	cnt_errors "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/errors"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/state"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/types"
)

var (
	errFake           = errors.New("fake error")
	errNotImplemented = errors.New("not implemented")
)

// Fake ingress
type fakeIngressLister struct {
	fails bool
	item  *api.Ingress
}

var _ lister.IngressLister = &fakeIngressLister{}

func newFakeIngressLister(fails bool, namespace, annotationManagedCertificates, annotationPreSharedCert string) *fakeIngressLister {
	ingressLister := &fakeIngressLister{
		fails: fails,
		item: &api.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
			},
		},
	}

	ingressLister.item.Annotations = map[string]string{
		annotationManagedCertificatesKey: annotationManagedCertificates,
		annotationPreSharedCertKey:       annotationPreSharedCert,
	}

	return ingressLister
}

func (f *fakeIngressLister) List(selector labels.Selector) ([]*api.Ingress, error) {
	if f.fails {
		return nil, errFake
	}

	return []*api.Ingress{f.item}, nil
}

func (f *fakeIngressLister) Ingresses(namespace string) lister.IngressNamespaceLister {
	return nil
}

// Fake state
type entry struct {
	sslCertificateName string
	softDeleted        bool
}

type fakeState struct {
	fails bool
	data  map[types.CertId]entry
}

var _ state.State = fakeState{}

func newFakeState(fails bool, data map[types.CertId]entry) fakeState {
	return fakeState{
		fails: fails,
		data:  data,
	}
}

func (f fakeState) Delete(id types.CertId) {
	// not implemented
}

func (f fakeState) ForeachKey(fun func(id types.CertId)) {
	for id := range f.data {
		fun(id)
	}
}

func (f fakeState) GetSslCertificateName(id types.CertId) (string, error) {
	if f.fails {
		return "", errFake
	}

	entry, e := f.data[id]
	if !e {
		return "", cnt_errors.ErrManagedCertificateNotFound
	}

	return entry.sslCertificateName, nil
}

func (f fakeState) IsSoftDeleted(id types.CertId) (bool, error) {
	if f.fails {
		return false, errFake
	}

	entry, e := f.data[id]
	if !e {
		return false, cnt_errors.ErrManagedCertificateNotFound
	}

	return entry.softDeleted, nil
}

func (f fakeState) IsSslCertificateCreationReported(id types.CertId) (bool, error) {
	return false, errNotImplemented
}

func (f fakeState) SetSslCertificateCreationReported(id types.CertId) error {
	return errNotImplemented
}

func (f fakeState) SetSslCertificateName(id types.CertId, sslCertificateName string) {
	// not implemented
}

func (f fakeState) SetSoftDeleted(id types.CertId) error {
	return errNotImplemented
}

func TestBindCertificates(t *testing.T) {
	ingressClient := &fake.FakeExtensionsV1beta1{Fake: &cgo_testing.Fake{}}
	ingressLister := newFakeIngressLister(false, "default", "mcrt1,mcrt2", "sslCert1")
	state := newFakeState(false, map[types.CertId]entry{
		types.NewCertId("default", "mcrt1"): entry{"sslCert1", true},
		types.NewCertId("default", "mcrt2"): entry{"sslCert2", false},
	})
	sut := New(ingressClient, ingressLister, state)

	sut.BindCertificates()

	preSharedCert := ingressLister.item.Annotations[annotationPreSharedCertKey]
	wantPreSharedCert := "sslCert2"
	if preSharedCert != wantPreSharedCert {
		t.Fatalf("Annotation pre-shared-cert: %s, want: %s", preSharedCert, wantPreSharedCert)
	}
}

func TestGetCertificatesFromState(t *testing.T) {
	for _, tc := range []struct {
		desc                            string
		stateFails                      bool
		stateEntries                    map[types.CertId]entry
		wantManagedCertificatesToAttach map[types.CertId]string
		wantSslCertificatesToDetach     map[string]bool
	}{
		{
			"State fails, empty entries passed, want empty output",
			true,
			nil,
			nil,
			nil,
		},
		{
			"State fails, non-empty entries passed, want empty output",
			true,
			map[types.CertId]entry{types.NewCertId("default", "a"): entry{"b", false}},
			nil,
			nil,
		},
		{
			"State succeeds, empty entries passed, want empty output",
			false,
			nil,
			nil,
			nil,
		},
		{
			"State succeeds, non-empty entries passed, want one to attach",
			false,
			map[types.CertId]entry{types.NewCertId("default", "mcrt1"): entry{"sslCert1", false}},
			map[types.CertId]string{types.NewCertId("default", "mcrt1"): "sslCert1"},
			nil,
		},
		{
			"State succeeds, non-empty entries passed, want one to detach",
			false,
			map[types.CertId]entry{types.NewCertId("default", "mcrt2"): entry{"sslCert2", true}},
			nil,
			map[string]bool{"sslCert2": true},
		},
		{
			"State succeeds, non-empty entries passed, want one to attach and one to detach",
			false,
			map[types.CertId]entry{
				types.NewCertId("default", "mcrt1"): entry{"sslCert1", false},
				types.NewCertId("default", "mcrt2"): entry{"sslCert2", true},
			},
			map[types.CertId]string{types.NewCertId("default", "mcrt1"): "sslCert1"},
			map[string]bool{"sslCert2": true},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			ingressClient := &fake.FakeExtensionsV1beta1{}
			sut := binderImpl{
				ingressClient: ingressClient,
				ingressLister: newFakeIngressLister(false, "", "", ""),
				state:         newFakeState(tc.stateFails, tc.stateEntries),
			}
			mcrtToAttach, sslCertToDetach := sut.getCertificatesFromState()

			if !reflect.DeepEqual(mcrtToAttach, tc.wantManagedCertificatesToAttach) {
				if len(mcrtToAttach) != 0 || len(tc.wantManagedCertificatesToAttach) != 0 {
					t.Fatalf("ManagedCertificates to attach: %#v, want: %#v", mcrtToAttach,
						tc.wantManagedCertificatesToAttach)
				}
			}

			if !reflect.DeepEqual(sslCertToDetach, tc.wantSslCertificatesToDetach) {
				if len(sslCertToDetach) != 0 || len(tc.wantSslCertificatesToDetach) != 0 {
					t.Fatalf("SslCertificates to detach: %#v, want: %#v", sslCertToDetach,
						tc.wantSslCertificatesToDetach)
				}
			}
		})
	}
}

func TestEnsureCertificatesAttached(t *testing.T) {
	for _, tc := range []struct {
		desc                          string
		ingressFails                  bool
		ingressNamespace              string
		annotationManagedCertificates string
		annotationPreSharedCert       string
		managedCertificatesToAttach   map[types.CertId]string
		sslCertificatesToDetach       map[string]bool
		wantErr                       bool
		wantAnnotationPreSharedCert   string
	}{
		{
			"Ingress list fails",
			true,
			"",
			"",
			"",
			nil,
			nil,
			true,
			"",
		},
		{
			"Ingress list succeeds, one certificate to attach is already attached",
			false,
			"default",
			"mcrt1",
			"sslCert1",
			map[types.CertId]string{types.NewCertId("default", "mcrt1"): "sslCert1"},
			nil,
			false,
			"sslCert1",
		},
		{
			"Ingress list succeeds, attaches one certificate",
			false,
			"default",
			"mcrt1",
			"",
			map[types.CertId]string{types.NewCertId("default", "mcrt1"): "sslCert1"},
			nil,
			false,
			"sslCert1",
		},
		{
			"Ingress list succeeds, detaches one certificate",
			false,
			"default",
			"",
			"sslCert2",
			nil,
			map[string]bool{"sslCert2": true},
			false,
			"",
		},
		{
			"Ingress list succeeds, fails to attach a certificate - different namespace",
			false,
			"foobar",
			"mcrt1",
			"",
			map[types.CertId]string{types.NewCertId("default", "mcrt1"): "sslCert1"},
			nil,
			false,
			"",
		},
		{
			"Ingress list succeeds, attaches one certificate, leaves the other one intact",
			false,
			"default",
			"mcrt1",
			"sslCertX",
			map[types.CertId]string{types.NewCertId("default", "mcrt1"): "sslCert1"},
			nil,
			false,
			"sslCertX,sslCert1",
		},
		{
			"Ingress list succeeds, attaches one certificate, detaches one, and leaves additional one intact",
			false,
			"default",
			"mcrt1",
			"sslCertX,sslCert2",
			map[types.CertId]string{types.NewCertId("default", "mcrt1"): "sslCert1"},
			map[string]bool{"sslCert2": true},
			false,
			"sslCertX,sslCert1",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			ingressClient := &fake.FakeExtensionsV1beta1{Fake: &cgo_testing.Fake{}}
			ingressLister := newFakeIngressLister(tc.ingressFails, tc.ingressNamespace, tc.annotationManagedCertificates,
				tc.annotationPreSharedCert)
			sut := binderImpl{
				ingressClient: ingressClient,
				ingressLister: ingressLister,
				state:         newFakeState(false, nil),
			}
			err := sut.ensureCertificatesAttached(tc.managedCertificatesToAttach, tc.sslCertificatesToDetach)

			if tc.wantErr && err == nil {
				t.Fatalf("Ensure certificates attached err: %v, want err: %t", err, tc.wantErr)
			}

			preSharedCert := ingressLister.item.Annotations[annotationPreSharedCertKey]
			if !reflect.DeepEqual(parse(preSharedCert), parse(tc.wantAnnotationPreSharedCert)) {
				t.Fatalf("Annotation pre-shared-cert: %s, want: %s", preSharedCert, tc.wantAnnotationPreSharedCert)
			}
		})
	}
}

func TestParse(t *testing.T) {
	for _, tc := range []struct {
		annotation string
		wantItems  []string
	}{
		{"", nil},
		{",", nil},
		{"a", []string{"a"}},
		{"a,", []string{"a"}},
		{",a", []string{"a"}},
		{" a ", []string{"a"}},
		{"a,b", []string{"a", "b"}},
		{" a , b ", []string{"a", "b"}},
	} {
		itemSet := parse(tc.annotation)
		var items []string
		for item := range itemSet {
			items = append(items, item)
		}
		sort.Strings(items)

		if !reflect.DeepEqual(items, tc.wantItems) {
			t.Fatalf("parse(%s) = %v, want %v", tc.annotation, items, tc.wantItems)
		}
	}
}
