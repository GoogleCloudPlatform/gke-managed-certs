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
	"errors"
	"fmt"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	compute "google.golang.org/api/compute/v1"
	apiv1beta1 "k8s.io/api/extensions/v1beta1"

	apisv1 "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/networking.gke.io/v1"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clients/event"
	clientsingress "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clients/ingress"
	clientsmcrt "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clients/managedcertificate"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/config"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/metrics"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/sslcertificatemanager"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/state"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/testhelper/ingress"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/testhelper/managedcertificate"
	utilserrors "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/errors"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/random"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/types"
)

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

func TestIngress(t *testing.T) {
	for description, tc := range map[string]struct {
		state       map[types.Id]state.Entry
		ingress     *apiv1beta1.Ingress
		wantIngress *apiv1beta1.Ingress
		wantEvent   event.Fake
		wantMetrics metrics.Fake
		wantErr     error
	}{
		"different namespace": {
			// A ManagedCertificate from in-a-different namespace
			// is attached to an Ingress from the default namespace.
			// Ingress is not processed.
			state: map[types.Id]state.Entry{
				types.NewId("in-a-different-namespace", "in-a-different-namespace"): state.Entry{SslCertificateName: "in-a-different-namespace"},
			},
			ingress:     ingress.New(types.NewId("default", "foo"), "in-a-different-namespace", ""),
			wantIngress: ingress.New(types.NewId("default", "foo"), "in-a-different-namespace", ""),
			wantEvent:   event.Fake{MissingCnt: 1},
			wantErr:     utilserrors.NotFound,
		},
		"not existing certificate": {
			// A not existing ManagedCertificate is attached to an Ingress
			// from the same namespace. Ingress is not processed.
			state:       map[types.Id]state.Entry{},
			ingress:     ingress.New(types.NewId("default", "foo"), "not-existing-certificate", ""),
			wantIngress: ingress.New(types.NewId("default", "foo"), "not-existing-certificate", ""),
			wantEvent:   event.Fake{MissingCnt: 1},
			wantErr:     utilserrors.NotFound,
		},
		"happy path": {
			state: map[types.Id]state.Entry{
				types.NewId("default", "regular1"): state.Entry{SslCertificateName: "regular1"},
				types.NewId("default", "regular2"): state.Entry{SslCertificateName: "regular2"},
				types.NewId("default", "deleted1"): state.Entry{SslCertificateName: "deleted1", SoftDeleted: true},
				types.NewId("default", "deleted2"): state.Entry{SslCertificateName: "deleted2", SoftDeleted: true},
			},
			ingress:     ingress.New(types.NewId("default", "foo"), "regular1,regular2,deleted1,deleted2", "regular1,deleted1"),
			wantIngress: ingress.New(types.NewId("default", "foo"), "regular1,regular2,deleted1,deleted2", "regular1,regular2"),
			wantMetrics: metrics.Fake{SslCertificateBindingLatencyObserved: 2},
		},
		"metrics: excluded from SLO calculation": {
			state: map[types.Id]state.Entry{
				types.NewId("default", "excludedSLO1"): state.Entry{SslCertificateName: "excludedSLO1", ExcludedFromSLO: true},
				types.NewId("default", "regular"):      state.Entry{SslCertificateName: "regular"},
				types.NewId("default", "excludedSLO2"): state.Entry{SslCertificateName: "excludedSLO2", ExcludedFromSLO: true},
			},
			ingress:     ingress.New(types.NewId("default", "foo"), "excludedSLO1,excludedSLO2,regular", "excludedSLO1"),
			wantIngress: ingress.New(types.NewId("default", "foo"), "excludedSLO1,excludedSLO2,regular", "excludedSLO1,excludedSLO2,regular"),
			wantMetrics: metrics.Fake{SslCertificateBindingLatencyObserved: 1},
		},
		"metrics: binding already reported": {
			state: map[types.Id]state.Entry{
				types.NewId("default", "bindingReported1"): state.Entry{SslCertificateName: "bindingReported1", SslCertificateBindingReported: true},
				types.NewId("default", "regular"):          state.Entry{SslCertificateName: "regular"},
				types.NewId("default", "bindingReported2"): state.Entry{SslCertificateName: "bindingReported2", SslCertificateBindingReported: true},
			},
			ingress:     ingress.New(types.NewId("default", "foo"), "bindingReported1,bindingReported2,regular", "bindingReported1"),
			wantIngress: ingress.New(types.NewId("default", "foo"), "bindingReported1,bindingReported2,regular", "bindingReported1,bindingReported2,regular"),
			wantMetrics: metrics.Fake{SslCertificateBindingLatencyObserved: 1},
		},
	} {
		t.Run(description, func(t *testing.T) {
			ctx := context.Background()

			event := &event.Fake{}
			var managedCertificates []*apisv1.ManagedCertificate
			for id := range tc.state {
				domain := fmt.Sprintf("mcrt-%s.example.com", id.String())
				managedCertificates = append(managedCertificates, managedcertificate.New(id, domain).Build())
			}
			ingress := clientsingress.NewFake([]*apiv1beta1.Ingress{tc.ingress})
			metrics := metrics.NewFake()

			sync := New(config.NewFakeCertificateStatusConfig(), event, ingress,
				clientsmcrt.NewFake(managedCertificates), metrics, random.NewFake("", nil),
				sslcertificatemanager.NewFake(), state.NewFakeWithEntries(tc.state))

			id := types.NewId("default", "foo")
			err := sync.Ingress(ctx, id)
			if err != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("sync.Ingress(): %v, want %v", err, tc.wantErr)
			}

			gotIngresses, _ := ingress.List()
			if diff := cmp.Diff([]*apiv1beta1.Ingress{tc.wantIngress}, gotIngresses); diff != "" {
				t.Fatalf("Diff Ingress (-want, +got): %s", diff)
			}
			if diff := cmp.Diff(&tc.wantEvent, event); diff != "" {
				t.Fatalf("Diff events (-want, +got): %s", diff)
			}
			if diff := cmp.Diff(&tc.wantMetrics, metrics); diff != "" {
				t.Fatalf("Diff metrics (-want, +got): %s", diff)
			}
		})
	}
}

func getSslCertificate(id types.Id, state state.Interface,
	sslManager sslcertificatemanager.Interface) (*compute.SslCertificate, error) {

	entry, err := state.Get(id)
	if err != nil {
		return nil, err
	}

	return sslManager.Get(entry.SslCertificateName, nil)
}

func TestManagedCertificate(t *testing.T) {
	for description, tc := range map[string]struct {
		managedCertificate *apisv1.ManagedCertificate
		state              state.Interface
		sslManager         sslcertificatemanager.Interface
		random             random.Interface

		wantState              state.Interface
		wantSslManager         sslcertificatemanager.Interface
		wantManagedCertificate *apisv1.ManagedCertificate
		wantMetrics            *metrics.Fake
		wantError              error
	}{
		"API server: not found, state: not found, GCP: not found": {
			state:      state.NewFake(),
			sslManager: sslcertificatemanager.NewFake(),
			random:     random.New(""),

			wantState:      state.NewFake(),
			wantSslManager: sslcertificatemanager.NewFake(),
			wantMetrics:    metrics.NewFake(),
		},
		"API server: not found, state: found, GCP: not found": {
			state: state.NewFakeWithEntries(map[types.Id]state.Entry{
				types.NewId("default", "foo"): state.Entry{SslCertificateName: "foo"},
			}),
			sslManager: sslcertificatemanager.NewFake(),
			random:     random.New(""),

			wantState:      state.NewFake(),
			wantSslManager: sslcertificatemanager.NewFake(),
			wantMetrics:    metrics.NewFake(),
		},
		"API server: not found, state: found, GCP: found": {
			state: state.NewFakeWithEntries(map[types.Id]state.Entry{
				types.NewId("default", "foo"): state.Entry{
					SslCertificateName: "foo",
				},
			}),
			sslManager: sslcertificatemanager.
				NewFakeWithEntry("foo", []string{"example.com"},
					"ACTIVE", []string{"ACTIVE"}),
			random: random.New(""),

			wantState:      state.NewFake(),
			wantSslManager: sslcertificatemanager.NewFake(),
			wantMetrics:    metrics.NewFake(),
		},
		"API server: not found, state: found soft deleted, GCP: found": {
			state: state.NewFakeWithEntries(map[types.Id]state.Entry{
				types.NewId("default", "foo"): state.Entry{
					SslCertificateName: "foo",
					SoftDeleted:        true,
				},
			}),
			sslManager: sslcertificatemanager.
				NewFakeWithEntry("foo", []string{"example.com"},
					"ACTIVE", []string{"ACTIVE"}),
			random: random.New(""),

			wantState:      state.NewFake(),
			wantSslManager: sslcertificatemanager.NewFake(),
			wantMetrics:    metrics.NewFake(),
		},
		"API server: found, state: not found, GCP: not found": {
			managedCertificate: managedcertificate.
				New(types.NewId("default", "foo"), "example.com").Build(),
			state:      state.NewFake(),
			sslManager: sslcertificatemanager.NewFake(),
			random:     random.NewFake("foo", nil),

			wantState: state.NewFakeWithEntries(map[types.Id]state.Entry{
				types.NewId("default", "foo"): state.Entry{
					SslCertificateName:             "foo",
					SslCertificateCreationReported: true,
				},
			}),
			wantSslManager: sslcertificatemanager.
				NewFakeWithEntry("foo", []string{"example.com"}, "", nil),
			wantManagedCertificate: managedcertificate.
				New(types.NewId("default", "foo"), "example.com").
				WithCertificateName("foo").
				Build(),
			wantMetrics: &metrics.Fake{SslCertificateCreationLatencyObserved: 1},
		},
		"API server: found, state: not found, GCP: found": {
			managedCertificate: managedcertificate.
				New(types.NewId("default", "foo"), "example.com").Build(),
			state: state.NewFake(),
			sslManager: sslcertificatemanager.
				NewFakeWithEntry("foo", []string{"example.com"},
					"ACTIVE", []string{"ACTIVE"}),
			random: random.NewFake("foo", nil),

			wantState: state.NewFakeWithEntries(map[types.Id]state.Entry{
				types.NewId("default", "foo"): state.Entry{SslCertificateName: "foo"},
			}),
			wantSslManager: sslcertificatemanager.
				NewFakeWithEntry("foo", []string{"example.com"},
					"ACTIVE", []string{"ACTIVE"}),
			wantManagedCertificate: managedcertificate.
				New(types.NewId("default", "foo"), "example.com").
				WithStatus("Active", "Active").
				WithCertificateName("foo").
				Build(),
			wantMetrics: metrics.NewFake(),
		},
		"API server: found, state: found soft deleted, GCP: found": {
			managedCertificate: managedcertificate.
				New(types.NewId("default", "foo"), "example.com").Build(),
			state: state.NewFakeWithEntries(map[types.Id]state.Entry{
				types.NewId("default", "foo"): state.Entry{
					SslCertificateName: "foo",
					SoftDeleted:        true,
				},
			}),
			sslManager: sslcertificatemanager.
				NewFakeWithEntry("foo", []string{"example.com"},
					"ACTIVE", []string{"ACTIVE"}),
			random: random.NewFake("foo", nil),

			wantState:      state.NewFake(),
			wantSslManager: sslcertificatemanager.NewFake(),
			wantManagedCertificate: managedcertificate.
				New(types.NewId("default", "foo"), "example.com").Build(),
			wantMetrics: metrics.NewFake(),
		},
		"API server: found, state: found, GCP: found": {
			managedCertificate: managedcertificate.
				New(types.NewId("default", "foo"), "example.com").Build(),
			state: state.NewFakeWithEntries(map[types.Id]state.Entry{
				types.NewId("default", "foo"): state.Entry{
					SslCertificateName:             "foo",
					SslCertificateCreationReported: true,
				},
			}),
			sslManager: sslcertificatemanager.
				NewFakeWithEntry("foo", []string{"example.com"},
					"ACTIVE", []string{"ACTIVE"}),
			random: random.NewFake("foo", nil),

			wantState: state.NewFakeWithEntries(map[types.Id]state.Entry{
				types.NewId("default", "foo"): state.Entry{
					SslCertificateName:             "foo",
					SslCertificateCreationReported: true,
				},
			}),
			wantSslManager: sslcertificatemanager.
				NewFakeWithEntry("foo", []string{"example.com"},
					"ACTIVE", []string{"ACTIVE"}),
			wantManagedCertificate: managedcertificate.
				New(types.NewId("default", "foo"), "example.com").
				WithStatus("Active", "Active").
				WithCertificateName("foo").
				Build(),
			wantMetrics: metrics.NewFake(),
		},
		"API server: found, state: found soft deleted, GCP: not found": {
			managedCertificate: managedcertificate.
				New(types.NewId("default", "foo"), "example.com").Build(),
			state: state.NewFakeWithEntries(map[types.Id]state.Entry{
				types.NewId("default", "foo"): state.Entry{
					SslCertificateName: "foo",
					SoftDeleted:        true,
				},
			}),
			sslManager: sslcertificatemanager.NewFake(),
			random:     random.NewFake("foo", nil),

			wantState:      state.NewFake(),
			wantSslManager: sslcertificatemanager.NewFake(),
			wantManagedCertificate: managedcertificate.
				New(types.NewId("default", "foo"), "example.com").Build(),
			wantMetrics: metrics.NewFake(),
		},
		"API server: found, state: found reported, GCP: not found": {
			managedCertificate: managedcertificate.
				New(types.NewId("default", "foo"), "example.com").Build(),
			state: state.NewFakeWithEntries(map[types.Id]state.Entry{
				types.NewId("default", "foo"): state.Entry{
					SslCertificateName:             "foo",
					SslCertificateCreationReported: true,
				},
			}),
			sslManager: sslcertificatemanager.NewFake(),
			random:     random.NewFake("foo", nil),

			wantState: state.NewFakeWithEntries(map[types.Id]state.Entry{
				types.NewId("default", "foo"): state.Entry{
					SslCertificateName:             "foo",
					SslCertificateCreationReported: true,
				},
			}),
			wantSslManager: sslcertificatemanager.
				NewFakeWithEntry("foo", []string{"example.com"}, "", nil),
			wantManagedCertificate: managedcertificate.
				New(types.NewId("default", "foo"), "example.com").
				WithCertificateName("foo").
				Build(),
			wantMetrics: metrics.NewFake(),
		},
		"API server: found, state: found excluded from SLO, GCP: not found": {
			managedCertificate: managedcertificate.
				New(types.NewId("default", "foo"), "example.com").Build(),
			state: state.NewFakeWithEntries(map[types.Id]state.Entry{
				types.NewId("default", "foo"): state.Entry{
					SslCertificateName: "foo",
					ExcludedFromSLO:    true,
				},
			}),
			sslManager: sslcertificatemanager.NewFake(),
			random:     random.NewFake("foo", nil),

			wantState: state.NewFakeWithEntries(map[types.Id]state.Entry{
				types.NewId("default", "foo"): state.Entry{
					SslCertificateName: "foo",
					ExcludedFromSLO:    true,
				},
			}),
			wantSslManager: sslcertificatemanager.
				NewFakeWithEntry("foo", []string{"example.com"}, "", nil),
			wantManagedCertificate: managedcertificate.
				New(types.NewId("default", "foo"), "example.com").
				WithCertificateName("foo").
				Build(),
			wantMetrics: metrics.NewFake(),
		},
		"API server: found, state: found not reported, GCP: not found": {
			managedCertificate: managedcertificate.
				New(types.NewId("default", "foo"), "example.com").Build(),
			state: state.NewFakeWithEntries(map[types.Id]state.Entry{
				types.NewId("default", "foo"): state.Entry{SslCertificateName: "foo"},
			}),
			sslManager: sslcertificatemanager.NewFake(),
			random:     random.NewFake("foo", nil),

			wantState: state.NewFakeWithEntries(map[types.Id]state.Entry{
				types.NewId("default", "foo"): state.Entry{
					SslCertificateName:             "foo",
					SslCertificateCreationReported: true,
				},
			}),
			wantSslManager: sslcertificatemanager.
				NewFakeWithEntry("foo", []string{"example.com"}, "", nil),
			wantManagedCertificate: managedcertificate.
				New(types.NewId("default", "foo"), "example.com").
				WithCertificateName("foo").
				Build(),
			wantMetrics: &metrics.Fake{SslCertificateCreationLatencyObserved: 1},
		},
		"API server: found, state: found, GCP: found; certificates different": {
			managedCertificate: managedcertificate.
				New(types.NewId("default", "foo"), "example.com").Build(),
			state: state.NewFakeWithEntries(map[types.Id]state.Entry{
				types.NewId("default", "foo"): state.Entry{
					SslCertificateName:             "foo",
					SslCertificateCreationReported: true,
				},
			}),
			sslManager: sslcertificatemanager.
				NewFakeWithEntry("foo", []string{"different-domain.com"},
					"ACTIVE", []string{"ACTIVE"}),
			random: random.NewFake("foo", nil),

			wantState:      state.NewFake(),
			wantSslManager: sslcertificatemanager.NewFake(),
			wantManagedCertificate: managedcertificate.
				New(types.NewId("default", "foo"), "example.com").
				Build(),
			wantMetrics: metrics.NewFake(),
			wantError:   utilserrors.OutOfSync,
		},
	} {
		t.Run(description, func(t *testing.T) {
			ctx := context.Background()

			var managedCertificates []*apisv1.ManagedCertificate
			if tc.managedCertificate != nil {
				managedCertificates = append(managedCertificates,
					tc.managedCertificate)
			}

			config := config.NewFakeCertificateStatusConfig()
			managedCertificate := clientsmcrt.NewFake(managedCertificates)
			metrics := metrics.NewFake()
			sync := New(config, &event.Fake{}, clientsingress.NewFake(nil),
				managedCertificate, metrics, tc.random, tc.sslManager, tc.state)

			id := types.NewId("default", "foo")
			if err := sync.ManagedCertificate(ctx, id); err != tc.wantError {
				t.Fatalf("sync.ManagedCertificate(%s): %v, want %v",
					id, err, tc.wantError)
			}

			if diff := cmp.Diff(tc.wantState.List(), tc.state.List()); diff != "" {
				t.Fatalf("Diff state (-want, +got): %s", diff)
			}

			wantSslCertificate, wantSslCertificateErr := getSslCertificate(id,
				tc.wantState, tc.wantSslManager)
			gotSslCertificate, gotSslCertificateErr := getSslCertificate(id,
				tc.state, tc.sslManager)
			sslCertificateDiff := cmp.Diff(wantSslCertificate, gotSslCertificate)
			if wantSslCertificateErr != gotSslCertificateErr ||
				sslCertificateDiff != "" {

				t.Fatalf(`Diff SslCertificate (-want, +got): %s,
					got error: %v, want error: %v`,
					sslCertificateDiff, gotSslCertificateErr,
					wantSslCertificateErr)
			}

			if tc.wantManagedCertificate != nil && len(managedCertificates) != 1 {
				t.Fatalf(`ManagedCertificate nil, want %+v;
					total number of certificates: %d, want 1`,
					tc.wantManagedCertificate, len(managedCertificates))
			} else if tc.wantManagedCertificate == nil &&
				len(managedCertificates) != 0 {

				t.Fatalf(`ManagedCertificate %+v, want nil;
					total number of certificates: %d, want 0`,
					managedCertificates[0], len(managedCertificates))
			} else if len(managedCertificates) > 0 {
				if diff := cmp.Diff(tc.wantManagedCertificate,
					managedCertificates[0]); diff != "" {

					t.Fatalf("Diff ManagedCertificates (-want, +got): %s",
						diff)
				}
			}

			if diff := cmp.Diff(tc.wantMetrics, metrics); diff != "" {
				t.Fatalf("Diff metrics (-want, +got): %s", diff)
			}
		})
	}
}
