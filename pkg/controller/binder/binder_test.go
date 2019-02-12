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

	mcrt_api "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/networking.gke.io/v1beta1"
	cnt_errors "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/errors"
	cnt_fake "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/fake"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/types"
)

var (
	errFake = errors.New("fake error")
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

func TestBindCertificates(t *testing.T) {
	ingressClient := &fake.FakeExtensionsV1beta1{Fake: &cgo_testing.Fake{}}
	ingressLister := newFakeIngressLister(false, "default", "mcrt1,mcrt2", "sslCert1")

	mcrt1 := types.NewCertId("default", "mcrt1")
	mcrt2 := types.NewCertId("default", "mcrt2")
	mcrtLister := cnt_fake.NewLister(nil, []*mcrt_api.ManagedCertificate{
		cnt_fake.NewManagedCertificate(mcrt1, "foo.com"),
		cnt_fake.NewManagedCertificate(mcrt2, "bar.com"),
	})
	state := cnt_fake.NewStateWithEntries(map[types.CertId]cnt_fake.StateEntry{
		mcrt1: cnt_fake.StateEntry{SslCertificateName: "sslCert1", SoftDeleted: true},
		mcrt2: cnt_fake.StateEntry{SslCertificateName: "sslCert2", SoftDeleted: false},
	})

	metrics := cnt_fake.NewMetrics()
	sut := New(ingressClient, ingressLister, mcrtLister, metrics, state)

	sut.BindCertificates()

	preSharedCert := ingressLister.item.Annotations[annotationPreSharedCertKey]
	wantPreSharedCert := "sslCert2"
	if preSharedCert != wantPreSharedCert {
		t.Fatalf("Annotation pre-shared-cert: %s, want: %s", preSharedCert, wantPreSharedCert)
	}

	if metrics.SslCertificateBindingLatencyObserved != 1 {
		t.Fatalf("SslCertificate binding metric reported %d times, want 1", metrics.SslCertificateBindingLatencyObserved)
	}

	if reported, err := state.IsSslCertificateBindingReported(mcrt2); err != nil || !reported {
		t.Fatalf("SslCertificate binding metric for %s set in state err: %v, reported: %t, want true", mcrt2.String(), err,
			reported)
	}
}

func TestGetCertificatesFromState(t *testing.T) {
	for _, tc := range []struct {
		desc                            string
		stateEntries                    map[types.CertId]cnt_fake.StateEntry
		wantManagedCertificatesToAttach map[types.CertId]string
		wantSslCertificatesToDetach     map[string]bool
	}{
		{
			"Empty entries passed, want empty output",
			nil,
			nil,
			nil,
		},
		{
			"Non-empty failing entries passed, want empty output",
			map[types.CertId]cnt_fake.StateEntry{
				types.NewCertId("default", "a"): cnt_fake.StateEntry{
					SslCertificateName: "b",
					SoftDeletedErr:     cnt_errors.ErrManagedCertificateNotFound,
				},
			},
			nil,
			nil,
		},
		{
			"Empty entries passed, want empty output",
			nil,
			nil,
			nil,
		},
		{
			"Non-empty entries passed, want one to attach",
			map[types.CertId]cnt_fake.StateEntry{
				types.NewCertId("default", "mcrt1"): cnt_fake.StateEntry{SslCertificateName: "sslCert1"},
			},
			map[types.CertId]string{types.NewCertId("default", "mcrt1"): "sslCert1"},
			nil,
		},
		{
			"Non-empty entries passed, want one to detach",
			map[types.CertId]cnt_fake.StateEntry{
				types.NewCertId("default", "mcrt2"): cnt_fake.StateEntry{
					SslCertificateName: "sslCert2",
					SoftDeleted:        true,
				},
			},
			nil,
			map[string]bool{"sslCert2": true},
		},
		{
			"Non-empty entries passed, want one to attach and one to detach",
			map[types.CertId]cnt_fake.StateEntry{
				types.NewCertId("default", "mcrt1"): cnt_fake.StateEntry{SslCertificateName: "sslCert1"},
				types.NewCertId("default", "mcrt2"): cnt_fake.StateEntry{
					SslCertificateName: "sslCert2",
					SoftDeleted:        true,
				},
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
				state:         cnt_fake.NewStateWithEntries(tc.stateEntries),
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
	mcrtId1 := types.NewCertId("default", "mcrt1")
	mcrtId2 := types.NewCertId("foobar", "mcrt2")

	for _, tc := range []struct {
		desc                          string
		ingressFails                  bool
		ingressNamespace              string
		annotationManagedCertificates string
		annotationPreSharedCert       string
		managedCertificatesToAttach   map[types.CertId]string
		sslCertificatesToDetach       map[string]bool
		stateEntries                  map[types.CertId]cnt_fake.StateEntry
		wantErr                       bool
		wantAnnotationPreSharedCert   string
		wantBindingLatencyObservedFor []types.CertId
	}{
		{
			desc:         "Ingress list fails",
			ingressFails: true,
			wantErr:      true,
		},
		{
			desc:                          "Ingress list succeeds, one certificate to attach is already attached",
			ingressNamespace:              "default",
			annotationManagedCertificates: "mcrt1",
			annotationPreSharedCert:       "sslCert1",
			managedCertificatesToAttach:   map[types.CertId]string{mcrtId1: "sslCert1"},
			wantAnnotationPreSharedCert:   "sslCert1",
		},
		{
			desc:                          "Ingress list succeeds, attaches one certificate",
			ingressNamespace:              "default",
			annotationManagedCertificates: "mcrt1",
			managedCertificatesToAttach:   map[types.CertId]string{mcrtId1: "sslCert1"},
			stateEntries:                  map[types.CertId]cnt_fake.StateEntry{mcrtId1: cnt_fake.StateEntry{}},
			wantAnnotationPreSharedCert:   "sslCert1",
			wantBindingLatencyObservedFor: []types.CertId{mcrtId1},
		},
		{
			desc:                    "Ingress list succeeds, detaches one certificate",
			ingressNamespace:        "default",
			annotationPreSharedCert: "sslCert2",
			sslCertificatesToDetach: map[string]bool{"sslCert2": true},
		},
		{
			desc:                          "Ingress list succeeds, fails to attach a certificate - different namespace",
			ingressNamespace:              "foobar",
			annotationManagedCertificates: "mcrt1",
			managedCertificatesToAttach:   map[types.CertId]string{mcrtId1: "sslCert1"},
		},
		{
			desc:                          "Ingress list succeeds, attaches one certificate - fails to attach another - different namespace",
			ingressNamespace:              "foobar",
			annotationManagedCertificates: "mcrt1,mcrt2",
			managedCertificatesToAttach:   map[types.CertId]string{mcrtId1: "sslCert1", mcrtId2: "sslCert2"},
			stateEntries: map[types.CertId]cnt_fake.StateEntry{
				mcrtId1: cnt_fake.StateEntry{},
				mcrtId2: cnt_fake.StateEntry{},
			},
			wantAnnotationPreSharedCert:   "sslCert2",
			wantBindingLatencyObservedFor: []types.CertId{mcrtId2},
		},
		{
			desc:                          "Ingress list succeeds, attaches one certificate, leaves other one intact",
			ingressNamespace:              "default",
			annotationManagedCertificates: "mcrt1",
			annotationPreSharedCert:       "sslCertX",
			managedCertificatesToAttach:   map[types.CertId]string{mcrtId1: "sslCert1"},
			stateEntries:                  map[types.CertId]cnt_fake.StateEntry{mcrtId1: cnt_fake.StateEntry{}},
			wantAnnotationPreSharedCert:   "sslCertX,sslCert1",
			wantBindingLatencyObservedFor: []types.CertId{mcrtId1},
		},
		{
			desc:                          "Ingress list succeeds, attaches one certificate, detaches one, and leaves additional one intact",
			ingressNamespace:              "default",
			annotationManagedCertificates: "mcrt1",
			annotationPreSharedCert:       "sslCertX,sslCert2",
			managedCertificatesToAttach:   map[types.CertId]string{mcrtId1: "sslCert1"},
			sslCertificatesToDetach:       map[string]bool{"sslCert2": true},
			stateEntries:                  map[types.CertId]cnt_fake.StateEntry{mcrtId1: cnt_fake.StateEntry{}},
			wantAnnotationPreSharedCert:   "sslCertX,sslCert1",
			wantBindingLatencyObservedFor: []types.CertId{mcrtId1},
		},
		{
			desc:                          "Ingress list succeeds, attaches one certificate failing to determine if excluded from SLO calculation",
			ingressNamespace:              "default",
			annotationManagedCertificates: "mcrt1",
			managedCertificatesToAttach:   map[types.CertId]string{mcrtId1: "sslCert1"},
			stateEntries:                  map[types.CertId]cnt_fake.StateEntry{mcrtId1: cnt_fake.StateEntry{ExcludedFromSLOErr: errFake}},
			wantAnnotationPreSharedCert:   "sslCert1",
			wantErr:                       true,
		},
		{
			desc:                          "Ingress list succeeds, attaches one certificate excluded from SLO calculation",
			ingressNamespace:              "default",
			annotationManagedCertificates: "mcrt1",
			managedCertificatesToAttach:   map[types.CertId]string{mcrtId1: "sslCert1"},
			stateEntries:                  map[types.CertId]cnt_fake.StateEntry{mcrtId1: cnt_fake.StateEntry{ExcludedFromSLO: true}},
			wantAnnotationPreSharedCert:   "sslCert1",
		},
		{
			desc:                          "Ingress list succeeds, attaches one certificate not excluded from SLO calculation",
			ingressNamespace:              "default",
			annotationManagedCertificates: "mcrt1",
			managedCertificatesToAttach:   map[types.CertId]string{mcrtId1: "sslCert1"},
			stateEntries:                  map[types.CertId]cnt_fake.StateEntry{mcrtId1: cnt_fake.StateEntry{}},
			wantAnnotationPreSharedCert:   "sslCert1",
			wantBindingLatencyObservedFor: []types.CertId{mcrtId1},
		},
		{
			desc: `Ingress list succeeds, attaches one certificate not excluded from SLO calculation;
				failure to determine if SslCertificate binding metric has been already reported`,
			ingressNamespace:              "default",
			annotationManagedCertificates: "mcrt1",
			managedCertificatesToAttach:   map[types.CertId]string{mcrtId1: "sslCert1"},
			stateEntries:                  map[types.CertId]cnt_fake.StateEntry{mcrtId1: cnt_fake.StateEntry{SslCertificateBindingErr: errFake}},
			wantAnnotationPreSharedCert:   "sslCert1",
			wantErr:                       true,
		},
		{
			desc: `Ingress list succeeds, attaches one certificate not excluded from SLO calculation;
				SslCertificate binding metric has been already reported`,
			ingressNamespace:              "default",
			annotationManagedCertificates: "mcrt1",
			managedCertificatesToAttach:   map[types.CertId]string{mcrtId1: "sslCert1"},
			stateEntries:                  map[types.CertId]cnt_fake.StateEntry{mcrtId1: cnt_fake.StateEntry{SslCertificateBindingReported: true}},
			wantAnnotationPreSharedCert:   "sslCert1",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			ingressClient := &fake.FakeExtensionsV1beta1{Fake: &cgo_testing.Fake{}}
			ingressLister := newFakeIngressLister(tc.ingressFails, tc.ingressNamespace, tc.annotationManagedCertificates,
				tc.annotationPreSharedCert)
			mcrtLister := cnt_fake.NewLister(nil, []*mcrt_api.ManagedCertificate{
				cnt_fake.NewManagedCertificate(mcrtId1, "foo.com"),
				cnt_fake.NewManagedCertificate(mcrtId2, "bar.com"),
			})
			metrics := cnt_fake.NewMetrics()
			state := cnt_fake.NewStateWithEntries(tc.stateEntries)
			sut := binderImpl{
				ingressClient: ingressClient,
				ingressLister: ingressLister,
				mcrtLister:    mcrtLister,
				metrics:       metrics,
				state:         state,
			}
			err := sut.ensureCertificatesAttached(tc.managedCertificatesToAttach, tc.sslCertificatesToDetach)

			if tc.wantErr == (err == nil) {
				t.Fatalf("Ensure certificates attached err: %v, want err: %t", err, tc.wantErr)
			}

			preSharedCert := ingressLister.item.Annotations[annotationPreSharedCertKey]
			if !reflect.DeepEqual(parse(preSharedCert), parse(tc.wantAnnotationPreSharedCert)) {
				t.Fatalf("Annotation pre-shared-cert: %s, want: %s", preSharedCert, tc.wantAnnotationPreSharedCert)
			}

			if len(tc.wantBindingLatencyObservedFor) != metrics.SslCertificateBindingLatencyObserved {
				t.Fatalf("SslCertificate binding latency observed %d times, want %d times",
					metrics.SslCertificateBindingLatencyObserved, len(tc.wantBindingLatencyObservedFor))
			}

			for _, id := range tc.wantBindingLatencyObservedFor {
				if reported, err := state.IsSslCertificateBindingReported(id); err != nil || !reported {
					t.Fatalf("SslCertificate binding latency metric in state: %t (want true), err: %v (want nil)",
						reported, err)
				}
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
