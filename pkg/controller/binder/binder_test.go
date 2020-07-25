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

package binder

import (
	"errors"
	"fmt"
	"sort"
	"testing"

	api "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes/typed/extensions/v1beta1/fake"
	lister "k8s.io/client-go/listers/extensions/v1beta1"
	cgotesting "k8s.io/client-go/testing"

	apisv1beta2 "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/networking.gke.io/v1beta2"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clients/event"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/metrics"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/state"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/testhelper/managedcertificate"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/types"
	"github.com/google/go-cmp/cmp"
)

// Fake ingress lister
type fakeIngressLister struct {
	err   error
	items []*api.Ingress
}

var _ lister.IngressLister = &fakeIngressLister{}

func newFakeIngressLister(err error, items []*api.Ingress) *fakeIngressLister {
	return &fakeIngressLister{err: err, items: items}
}

func (f *fakeIngressLister) List(selector labels.Selector) ([]*api.Ingress, error) {
	if f.err != nil {
		return nil, f.err
	}

	return f.items, nil
}

func (f *fakeIngressLister) Ingresses(namespace string) lister.IngressNamespaceLister {
	return nil
}

func TestBindCertificates_IngressListerFailing(t *testing.T) {
	t.Parallel()

	errLister := errors.New("ingress lister test error")
	mcrtLister := managedcertificate.NewLister(nil)
	ingressClient := &fake.FakeExtensionsV1beta1{Fake: &cgotesting.Fake{}}
	ingressLister := newFakeIngressLister(errLister, nil)

	binder := New(&event.FakeEvent{}, ingressClient, ingressLister,
		mcrtLister, metrics.NewFake(), state.NewFake())

	if err := binder.BindCertificates(); err != errLister {
		t.Fatalf("binder.BindCertificates(): %v, want %v", err, errLister)
	}
}

func TestBindCertificates(t *testing.T) {
	t.Parallel()

	for description, tc := range map[string]struct {
		state         map[types.CertId]state.Entry
		ingresses     []*api.Ingress
		wantIngresses []*api.Ingress
		wantEvent     event.FakeEvent
		wantMetrics   metrics.Fake
	}{
		"different namespace": {
			// A ManagedCertificate from in-a-different namespace is attached to an Ingress
			// from the default namespace. Ingress is not processed.
			state: map[types.CertId]state.Entry{
				types.NewCertId("in-a-different-namespace", "in-a-different-namespace"): state.Entry{SslCertificateName: "in-a-different-namespace"},
			},
			ingresses: []*api.Ingress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "default",
						Annotations: map[string]string{
							annotationManagedCertificatesKey: "in-a-different-namespace",
							annotationPreSharedCertKey:       "",
						},
					},
				},
			},
			wantIngresses: []*api.Ingress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "default",
						Annotations: map[string]string{
							annotationManagedCertificatesKey: "in-a-different-namespace",
							annotationPreSharedCertKey:       "",
						},
					},
				},
			},
			wantEvent: event.FakeEvent{MissingCnt: 1},
		},
		"not existing certificate": {
			// A not existing ManagedCertificate is attached to an Ingress from the same
			// namespace. Ingress is not processed.
			state: map[types.CertId]state.Entry{},
			ingresses: []*api.Ingress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "default",
						Annotations: map[string]string{
							annotationManagedCertificatesKey: "not-existing-certificate",
							annotationPreSharedCertKey:       "",
						},
					},
				},
			},
			wantIngresses: []*api.Ingress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "default",
						Annotations: map[string]string{
							annotationManagedCertificatesKey: "not-existing-certificate",
							annotationPreSharedCertKey:       "",
						},
					},
				},
			},
			wantEvent: event.FakeEvent{MissingCnt: 1},
		},
		"ingresses are processed independently": {
			// A missing (not existing) ManagedCertificate is attached to the first Ingress,
			// the namespaces match - processing the first Ingress fails once the certificate
			// cannot be found. A valid ManagedCertificate is attached to the second Ingress
			// which is successfully processed.
			state: map[types.CertId]state.Entry{
				types.NewCertId("default", "regular"): state.Entry{SslCertificateName: "regular"},
			},
			ingresses: []*api.Ingress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "default",
						Annotations: map[string]string{
							annotationManagedCertificatesKey: "not-existing-certificate",
							annotationPreSharedCertKey:       "",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "default",
						Annotations: map[string]string{
							annotationManagedCertificatesKey: "regular",
							annotationPreSharedCertKey:       "",
						},
					},
				},
			},
			wantIngresses: []*api.Ingress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "default",
						Annotations: map[string]string{
							annotationManagedCertificatesKey: "not-existing-certificate",
							annotationPreSharedCertKey:       "",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "default",
						Annotations: map[string]string{
							annotationManagedCertificatesKey: "regular",
							annotationPreSharedCertKey:       "regular",
						},
					},
				},
			},
			wantEvent:   event.FakeEvent{MissingCnt: 1},
			wantMetrics: metrics.Fake{SslCertificateBindingLatencyObserved: 1},
		},
		"happy path": {
			state: map[types.CertId]state.Entry{
				types.NewCertId("default", "regular1"): state.Entry{SslCertificateName: "regular1"},
				types.NewCertId("default", "regular2"): state.Entry{SslCertificateName: "regular2"},
				types.NewCertId("default", "deleted1"): state.Entry{SslCertificateName: "deleted1", SoftDeleted: true},
				types.NewCertId("default", "deleted2"): state.Entry{SslCertificateName: "deleted2", SoftDeleted: true},
			},
			ingresses: []*api.Ingress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "default",
						Annotations: map[string]string{
							annotationManagedCertificatesKey: "regular1,regular2,deleted1,deleted2",
							annotationPreSharedCertKey:       "regular1,deleted1",
						},
					},
				},
			},
			wantIngresses: []*api.Ingress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "default",
						Annotations: map[string]string{
							annotationManagedCertificatesKey: "regular1,regular2,deleted1,deleted2",
							annotationPreSharedCertKey:       "regular1,regular2",
						},
					},
				},
			},
			wantMetrics: metrics.Fake{SslCertificateBindingLatencyObserved: 2},
		},
		"metrics: excluded from SLO calculation": {
			state: map[types.CertId]state.Entry{
				types.NewCertId("default", "excludedSLO1"): state.Entry{SslCertificateName: "excludedSLO1", ExcludedFromSLO: true},
				types.NewCertId("default", "regular"):      state.Entry{SslCertificateName: "regular"},
				types.NewCertId("default", "excludedSLO2"): state.Entry{SslCertificateName: "excludedSLO2", ExcludedFromSLO: true},
			},
			ingresses: []*api.Ingress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "default",
						Annotations: map[string]string{
							annotationManagedCertificatesKey: "excludedSLO1,excludedSLO2,regular",
							annotationPreSharedCertKey:       "excludedSLO1",
						},
					},
				},
			},
			wantIngresses: []*api.Ingress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "default",
						Annotations: map[string]string{
							annotationManagedCertificatesKey: "excludedSLO1,excludedSLO2,regular",
							annotationPreSharedCertKey:       "excludedSLO1,excludedSLO2,regular",
						},
					},
				},
			},
			wantMetrics: metrics.Fake{SslCertificateBindingLatencyObserved: 1},
		},
		"metrics: binding already reported": {
			state: map[types.CertId]state.Entry{
				types.NewCertId("default", "bindingReported1"): state.Entry{SslCertificateName: "bindingReported1", SslCertificateBindingReported: true},
				types.NewCertId("default", "regular"):          state.Entry{SslCertificateName: "regular"},
				types.NewCertId("default", "bindingReported2"): state.Entry{SslCertificateName: "bindingReported2", SslCertificateBindingReported: true},
			},
			ingresses: []*api.Ingress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "default",
						Annotations: map[string]string{
							annotationManagedCertificatesKey: "bindingReported1,bindingReported2,regular",
							annotationPreSharedCertKey:       "bindingReported1",
						},
					},
				},
			},
			wantIngresses: []*api.Ingress{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "default",
						Annotations: map[string]string{
							annotationManagedCertificatesKey: "bindingReported1,bindingReported2,regular",
							annotationPreSharedCertKey:       "bindingReported1,bindingReported2,regular",
						},
					},
				},
			},
			wantMetrics: metrics.Fake{SslCertificateBindingLatencyObserved: 1},
		},
	} {
		t.Run(description, func(t *testing.T) {
			event := &event.FakeEvent{}
			var managedCertificates []*apisv1beta2.ManagedCertificate
			for id := range tc.state {
				domain := fmt.Sprintf("mcrt-%s.example.com", id.String())
				managedCertificates = append(managedCertificates, managedcertificate.New(id, domain).Build())
			}
			mcrtLister := managedcertificate.NewLister(managedCertificates)
			ingressClient := &fake.FakeExtensionsV1beta1{Fake: &cgotesting.Fake{}}
			ingressLister := newFakeIngressLister(nil, tc.ingresses)
			metrics := metrics.NewFake()

			binder := New(event, ingressClient, ingressLister, mcrtLister, metrics, state.NewFakeWithEntries(tc.state))

			if err := binder.BindCertificates(); err != nil {
				t.Fatalf("binder.BindCertificates(): %v, want nil", err)
			}

			if diff := cmp.Diff(tc.wantIngresses, ingressLister.items); diff != "" {
				t.Fatalf("binder.BindCertificates() -> ingress diff (-want, +got): %s", diff)
			}
			if diff := cmp.Diff(&tc.wantEvent, event); diff != "" {
				t.Fatalf("binder.BindCertificates() -> event diff (-want, +got): %s", diff)
			}
			if diff := cmp.Diff(&tc.wantMetrics, metrics); diff != "" {
				t.Fatalf("binder.BindCertificates() -> metrics diff (-want, +got): %s", diff)
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

		if diff := cmp.Diff(tc.wantItems, items); diff != "" {
			t.Fatalf("parse(%q): (-want, +got): %s", tc.annotation, diff)
		}
	}
}
