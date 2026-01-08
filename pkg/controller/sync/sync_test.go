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
	computev1 "google.golang.org/api/compute/v1"
	netv1 "k8s.io/api/networking/v1"

	v1 "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/networking.gke.io/v1"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clients/event"
	clientsingress "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clients/ingress"
	clientsmcrt "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clients/managedcertificate"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clients/ssl"
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
	t.Parallel()

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

func TestParseList(t *testing.T) {
	t.Parallel()

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
		{"b,a", []string{"b", "a"}},              // Order preserved
		{"c,a,b", []string{"c", "a", "b"}},       // Order preserved
		{"a,b,a", []string{"a", "b"}},            // Duplicates removed, first occurrence kept
		{"a,b,c,b,a", []string{"a", "b", "c"}},   // Multiple duplicates
		{" z , a , m ", []string{"z", "a", "m"}}, // Order preserved with spaces
	} {
		items := parseList(tc.annotation)

		if diff := cmp.Diff(tc.wantItems, items); diff != "" {
			t.Fatalf("parseList(%q): (-want, +got): %s", tc.annotation, diff)
		}
	}
}

func TestIngress(t *testing.T) {
	t.Parallel()

	for description, testCase := range map[string]struct {
		state       map[types.Id]state.Entry
		ingress     *netv1.Ingress
		wantIngress *netv1.Ingress
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
			ingress:     ingress.New(types.NewId("default", "foo"), ingress.AnnotationManagedCertificates("in-a-different-namespace")),
			wantIngress: ingress.New(types.NewId("default", "foo"), ingress.AnnotationManagedCertificates("in-a-different-namespace")),
			wantEvent:   event.Fake{MissingCertificateCnt: 1},
			wantErr:     utilserrors.NotFound,
		},
		"not existing certificate": {
			// A not existing ManagedCertificate is attached to an Ingress
			// from the same namespace. Ingress is not processed.
			state:       map[types.Id]state.Entry{},
			ingress:     ingress.New(types.NewId("default", "foo"), ingress.AnnotationManagedCertificates("not-existing-certificate")),
			wantIngress: ingress.New(types.NewId("default", "foo"), ingress.AnnotationManagedCertificates("not-existing-certificate")),
			wantEvent:   event.Fake{MissingCertificateCnt: 1},
			wantErr:     utilserrors.NotFound,
		},
		"ingress with nil annotations": {
			state:       map[types.Id]state.Entry{},
			ingress:     ingress.New(types.NewId("default", "foo")),
			wantIngress: ingress.New(types.NewId("default", "foo")),
		},
		"happy path": {
			state: map[types.Id]state.Entry{
				types.NewId("default", "regular1"): state.Entry{SslCertificateName: "regular1"},
				types.NewId("default", "regular2"): state.Entry{SslCertificateName: "regular2"},
				types.NewId("default", "deleted1"): state.Entry{SslCertificateName: "deleted1", SoftDeleted: true},
				types.NewId("default", "deleted2"): state.Entry{SslCertificateName: "deleted2", SoftDeleted: true},
			},
			ingress:     ingress.New(types.NewId("default", "foo"), ingress.AnnotationManagedCertificates("regular1,regular2,deleted1,deleted2"), ingress.AnnotationPreSharedCert("regular1,deleted1")),
			wantIngress: ingress.New(types.NewId("default", "foo"), ingress.AnnotationManagedCertificates("regular1,regular2,deleted1,deleted2"), ingress.AnnotationPreSharedCert("regular1,regular2")),
			wantMetrics: metrics.Fake{BindingCnt: 2},
		},
		"metrics: excluded from SLO calculation": {
			state: map[types.Id]state.Entry{
				types.NewId("default", "excludedSLO1"): state.Entry{SslCertificateName: "excludedSLO1", ExcludedFromSLO: true},
				types.NewId("default", "regular"):      state.Entry{SslCertificateName: "regular"},
				types.NewId("default", "excludedSLO2"): state.Entry{SslCertificateName: "excludedSLO2", ExcludedFromSLO: true},
			},
			ingress:     ingress.New(types.NewId("default", "foo"), ingress.AnnotationManagedCertificates("excludedSLO1,excludedSLO2,regular"), ingress.AnnotationPreSharedCert("excludedSLO1")),
			wantIngress: ingress.New(types.NewId("default", "foo"), ingress.AnnotationManagedCertificates("excludedSLO1,excludedSLO2,regular"), ingress.AnnotationPreSharedCert("excludedSLO1,excludedSLO2,regular")),
			wantMetrics: metrics.Fake{BindingCnt: 1},
		},
		"metrics: binding already reported": {
			state: map[types.Id]state.Entry{
				types.NewId("default", "bindingReported1"): state.Entry{SslCertificateName: "bindingReported1", SslCertificateBindingReported: true},
				types.NewId("default", "regular"):          state.Entry{SslCertificateName: "regular"},
				types.NewId("default", "bindingReported2"): state.Entry{SslCertificateName: "bindingReported2", SslCertificateBindingReported: true},
			},
			ingress:     ingress.New(types.NewId("default", "foo"), ingress.AnnotationManagedCertificates("bindingReported1,bindingReported2,regular"), ingress.AnnotationPreSharedCert("bindingReported1")),
			wantIngress: ingress.New(types.NewId("default", "foo"), ingress.AnnotationManagedCertificates("bindingReported1,bindingReported2,regular"), ingress.AnnotationPreSharedCert("bindingReported1,bindingReported2,regular")),
			wantMetrics: metrics.Fake{BindingCnt: 1},
		},
		"certificate order preserved: primary cert first": {
			// Tests that existing certificate order is preserved
			// User wants "primary" as the primary (fallback) certificate
			state: map[types.Id]state.Entry{
				types.NewId("default", "mcrt1"): state.Entry{SslCertificateName: "mcrt1-ssl"},
				types.NewId("default", "mcrt2"): state.Entry{SslCertificateName: "mcrt2-ssl"},
			},
			ingress:     ingress.New(types.NewId("default", "foo"), ingress.AnnotationManagedCertificates("mcrt1,mcrt2"), ingress.AnnotationPreSharedCert("primary,secondary")),
			wantIngress: ingress.New(types.NewId("default", "foo"), ingress.AnnotationManagedCertificates("mcrt1,mcrt2"), ingress.AnnotationPreSharedCert("primary,secondary,mcrt1-ssl,mcrt2-ssl")),
			wantMetrics: metrics.Fake{BindingCnt: 2},
		},
		"certificate order preserved: reverse alphabetical": {
			// Tests that non-alphabetical order is preserved
			// "zebra" comes first even though it's last alphabetically
			state: map[types.Id]state.Entry{
				types.NewId("default", "mcrt"): state.Entry{SslCertificateName: "mcrt-ssl"},
			},
			ingress:     ingress.New(types.NewId("default", "foo"), ingress.AnnotationManagedCertificates("mcrt"), ingress.AnnotationPreSharedCert("zebra,apple")),
			wantIngress: ingress.New(types.NewId("default", "foo"), ingress.AnnotationManagedCertificates("mcrt"), ingress.AnnotationPreSharedCert("zebra,apple,mcrt-ssl")),
			wantMetrics: metrics.Fake{BindingCnt: 1},
		},
		"certificate order preserved: new certs appended sorted": {
			// When adding multiple new managed certs, they should be sorted among themselves
			// but appended after existing certs
			state: map[types.Id]state.Entry{
				types.NewId("default", "cert-z"): state.Entry{SslCertificateName: "cert-z-ssl"},
				types.NewId("default", "cert-a"): state.Entry{SslCertificateName: "cert-a-ssl"},
				types.NewId("default", "cert-m"): state.Entry{SslCertificateName: "cert-m-ssl"},
			},
			ingress:     ingress.New(types.NewId("default", "foo"), ingress.AnnotationManagedCertificates("cert-z,cert-a,cert-m"), ingress.AnnotationPreSharedCert("existing")),
			wantIngress: ingress.New(types.NewId("default", "foo"), ingress.AnnotationManagedCertificates("cert-z,cert-a,cert-m"), ingress.AnnotationPreSharedCert("existing,cert-a-ssl,cert-m-ssl,cert-z-ssl")),
			wantMetrics: metrics.Fake{BindingCnt: 3},
		},
		"certificate combination: pre-shared order preserved + managed sorted": {
			// Demonstrates how certificates are combined:
			// 1. Pre-shared certificates preserve user's order (zebra,apple)
			// 2. Managed certificates are sorted alphabetically (cert-a-ssl,cert-m-ssl,cert-z-ssl)
			// 3. Final result: pre-shared first + managed appended
			state: map[types.Id]state.Entry{
				types.NewId("default", "cert-z"): state.Entry{SslCertificateName: "cert-z-ssl"},
				types.NewId("default", "cert-a"): state.Entry{SslCertificateName: "cert-a-ssl"},
				types.NewId("default", "cert-m"): state.Entry{SslCertificateName: "cert-m-ssl"},
			},
			ingress:     ingress.New(types.NewId("default", "foo"), ingress.AnnotationManagedCertificates("cert-z,cert-a,cert-m"), ingress.AnnotationPreSharedCert("zebra,apple")),
			wantIngress: ingress.New(types.NewId("default", "foo"), ingress.AnnotationManagedCertificates("cert-z,cert-a,cert-m"), ingress.AnnotationPreSharedCert("zebra,apple,cert-a-ssl,cert-m-ssl,cert-z-ssl")),
			wantMetrics: metrics.Fake{BindingCnt: 3},
		},
		"certificate combination: complex order preservation": {
			// Tests complex scenario with multiple pre-shared certs in non-alphabetical order
			// Shows that user's intended primary certificate (first in pre-shared) stays first
			state: map[types.Id]state.Entry{
				types.NewId("default", "managed-b"): state.Entry{SslCertificateName: "managed-b-ssl"},
				types.NewId("default", "managed-a"): state.Entry{SslCertificateName: "managed-a-ssl"},
			},
			ingress:     ingress.New(types.NewId("default", "foo"), ingress.AnnotationManagedCertificates("managed-b,managed-a"), ingress.AnnotationPreSharedCert("primary,secondary,fallback")),
			wantIngress: ingress.New(types.NewId("default", "foo"), ingress.AnnotationManagedCertificates("managed-b,managed-a"), ingress.AnnotationPreSharedCert("primary,secondary,fallback,managed-a-ssl,managed-b-ssl")),
			wantMetrics: metrics.Fake{BindingCnt: 2},
		},
		"certificate deduplication: same cert in pre-shared and managed": {
			// Tests that when the same certificate appears in both pre-shared and managed,
			// it's deduplicated and the pre-shared order is preserved
			state: map[types.Id]state.Entry{
				types.NewId("default", "mcrt1"): state.Entry{SslCertificateName: "shared-cert"}, // Same name as pre-shared
				types.NewId("default", "mcrt2"): state.Entry{SslCertificateName: "managed-only"},
			},
			ingress:     ingress.New(types.NewId("default", "foo"), ingress.AnnotationManagedCertificates("mcrt1,mcrt2"), ingress.AnnotationPreSharedCert("shared-cert,other-pre-shared")),
			wantIngress: ingress.New(types.NewId("default", "foo"), ingress.AnnotationManagedCertificates("mcrt1,mcrt2"), ingress.AnnotationPreSharedCert("shared-cert,other-pre-shared,managed-only")),
			wantMetrics: metrics.Fake{BindingCnt: 2},
		},
		"certificate order unchanged when no changes needed": {
			// Tests that when certificates are already correct, no update happens
			// This validates the comparison logic that prevents spurious updates
			// Note: BindingCnt is 0 because metrics are only reported when annotation is updated
			state: map[types.Id]state.Entry{
				types.NewId("default", "mcrt"): state.Entry{SslCertificateName: "mcrt-ssl"},
			},
			ingress:     ingress.New(types.NewId("default", "foo"), ingress.AnnotationManagedCertificates("mcrt"), ingress.AnnotationPreSharedCert("primary,mcrt-ssl")),
			wantIngress: ingress.New(types.NewId("default", "foo"), ingress.AnnotationManagedCertificates("mcrt"), ingress.AnnotationPreSharedCert("primary,mcrt-ssl")),
			wantMetrics: metrics.Fake{BindingCnt: 0},
		},
	} {
		testCase := testCase
		t.Run(description, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()

			event := &event.Fake{}
			var managedCertificates []*v1.ManagedCertificate
			for id := range testCase.state {
				domain := fmt.Sprintf("mcrt-%s.example.com", id.String())
				managedCertificates = append(managedCertificates, managedcertificate.New(id, domain).Build())
			}
			ingress := clientsingress.NewFake([]*netv1.Ingress{testCase.ingress})
			metrics := metrics.NewFake()
			state := state.NewFakeWithEntries(testCase.state)

			sync := New(config.NewFake(), event, ingress, clientsmcrt.NewFake(managedCertificates), metrics,
				random.NewFake(""), sslcertificatemanager.New(event, metrics, ssl.NewFake().Build(), state), state)

			id := types.NewId("default", "foo")
			err := sync.Ingress(ctx, id)
			if err != nil && !errors.Is(err, testCase.wantErr) {
				t.Fatalf("sync.Ingress(): %v, want %v", err, testCase.wantErr)
			}

			gotIngresses, _ := ingress.List()
			if diff := cmp.Diff([]*netv1.Ingress{testCase.wantIngress}, gotIngresses); diff != "" {
				t.Fatalf("Diff Ingress (-want, +got): %s", diff)
			}
			if diff := cmp.Diff(&testCase.wantEvent, event); diff != "" {
				t.Fatalf("Diff events (-want, +got): %s", diff)
			}
			if diff := cmp.Diff(&testCase.wantMetrics, metrics); diff != "" {
				t.Fatalf("Diff metrics (-want, +got): %s", diff)
			}
		})
	}
}

func getSslCert(id types.Id, state state.Interface, ssl ssl.Interface) (*computev1.SslCertificate, error) {
	entry, err := state.Get(id)
	if err != nil {
		return nil, err
	}

	return ssl.Get(entry.SslCertificateName)
}

func TestManagedCertificate(t *testing.T) {
	t.Parallel()

	for description, testCase := range map[string]struct {
		managedCertificate *v1.ManagedCertificate
		state              state.Interface
		ssl                ssl.Interface
		random             random.Interface

		wantState              state.Interface
		wantSsl                ssl.Interface
		wantManagedCertificate *v1.ManagedCertificate
		wantMetrics            *metrics.Fake
		wantError              error
	}{
		// API server found/not found: the ManagedCertificate exists/does not exist in the cluster.
		// State found/not found: the controller knows/does not know about a ManagedCertificate.
		// GCP found/not found: A corresponding SslCertificate resource exists/does not exist in GCP.
		"API server: not found, state: not found, GCP: not found": {
			state:  state.NewFake(),
			ssl:    ssl.NewFake().Build(),
			random: random.New(""),

			wantState:   state.NewFake(),
			wantSsl:     ssl.NewFake().Build(),
			wantMetrics: metrics.NewFake(),
		},
		"API server: not found, state: found, GCP: not found": {
			state: state.NewFakeWithEntries(map[types.Id]state.Entry{
				types.NewId("default", "foo"): state.Entry{SslCertificateName: "foo"},
			}),
			ssl:    ssl.NewFake().Build(),
			random: random.New(""),

			wantState:   state.NewFake(),
			wantSsl:     ssl.NewFake().Build(),
			wantMetrics: metrics.NewFake(),
		},
		"API server: not found, state: found, GCP: found": {
			state: state.NewFakeWithEntries(map[types.Id]state.Entry{
				types.NewId("default", "foo"): state.Entry{
					SslCertificateName: "foo",
				},
			}),
			ssl: ssl.NewFake().AddEntryWithStatus("foo", "ACTIVE",
				map[string]string{"example.com": "ACTIVE"}).Build(),
			random: random.New(""),

			wantState:   state.NewFake(),
			wantSsl:     ssl.NewFake().Build(),
			wantMetrics: metrics.NewFake(),
		},
		"API server: not found, state: found soft-deleted, GCP: found": {
			state: state.NewFakeWithEntries(map[types.Id]state.Entry{
				types.NewId("default", "foo"): state.Entry{
					SslCertificateName: "foo",
					SoftDeleted:        true,
				},
			}),
			ssl: ssl.NewFake().AddEntryWithStatus("foo", "ACTIVE",
				map[string]string{"example.com": "ACTIVE"}).Build(),
			random: random.New(""),

			wantState:   state.NewFake(),
			wantSsl:     ssl.NewFake().Build(),
			wantMetrics: metrics.NewFake(),
		},
		"API server: found, state: found soft-deleted, GCP: found": {
			managedCertificate: managedcertificate.
				New(types.NewId("default", "foo"), "example.com").Build(),
			state: state.NewFakeWithEntries(map[types.Id]state.Entry{
				types.NewId("default", "foo"): state.Entry{
					SslCertificateName: "foo",
					SoftDeleted:        true,
				},
			}),
			ssl: ssl.NewFake().AddEntryWithStatus("foo", "ACTIVE",
				map[string]string{"example.com": "ACTIVE"}).Build(),
			random: random.New(""),

			wantManagedCertificate: managedcertificate.
				New(types.NewId("default", "foo"), "example.com").
				WithStatus("Active", "Active").
				WithCertificateName("foo").
				Build(),
			wantState: state.NewFakeWithEntries(map[types.Id]state.Entry{
				types.NewId("default", "foo"): state.Entry{
					SslCertificateName: "foo",
					SoftDeleted:        false,
				},
			}),
			wantSsl: ssl.NewFake().AddEntryWithStatus("foo", "ACTIVE",
				map[string]string{"example.com": "ACTIVE"}).Build(),
			wantMetrics: metrics.NewFake(),
		},
		"API server: found, state: not found, GCP: not found": {
			managedCertificate: managedcertificate.
				New(types.NewId("default", "foo"), "example.com").Build(),
			state:  state.NewFake(),
			ssl:    ssl.NewFake().Build(),
			random: random.NewFake("foo"),

			wantState: state.NewFakeWithEntries(map[types.Id]state.Entry{
				types.NewId("default", "foo"): state.Entry{
					SslCertificateName:             "foo",
					SslCertificateCreationReported: true,
				},
			}),
			wantSsl: ssl.NewFake().AddEntry("foo", []string{"example.com"}).Build(),
			wantManagedCertificate: managedcertificate.
				New(types.NewId("default", "foo"), "example.com").
				WithCertificateName("foo").
				WithStatus("", "").
				Build(),
			wantMetrics: &metrics.Fake{CreationCnt: 1},
		},
		"API server: found, state: not found, GCP: found": {
			managedCertificate: managedcertificate.
				New(types.NewId("default", "foo"), "example.com").Build(),
			state: state.NewFake(),
			ssl: ssl.NewFake().AddEntryWithStatus("foo", "ACTIVE",
				map[string]string{"example.com": "ACTIVE"}).Build(),
			random: random.NewFake("bar"),

			wantState: state.NewFakeWithEntries(map[types.Id]state.Entry{
				types.NewId("default", "foo"): state.Entry{
					SslCertificateName:             "bar",
					SslCertificateCreationReported: true,
				},
			}),
			wantSsl: ssl.NewFake().AddEntry("bar", []string{"example.com"}).Build(),
			wantManagedCertificate: managedcertificate.
				New(types.NewId("default", "foo"), "example.com").
				WithStatus("", "").
				WithCertificateName("bar").
				Build(),
			wantMetrics: &metrics.Fake{CreationCnt: 1},
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
			ssl: ssl.NewFake().AddEntryWithStatus("foo", "ACTIVE",
				map[string]string{"example.com": "ACTIVE"}).Build(),
			random: random.NewFake("foo"),

			wantState: state.NewFakeWithEntries(map[types.Id]state.Entry{
				types.NewId("default", "foo"): state.Entry{
					SslCertificateName: "foo",
					SoftDeleted:        false,
				},
			}),
			wantSsl: ssl.NewFake().AddEntryWithStatus("foo", "ACTIVE",
				map[string]string{"example.com": "ACTIVE"}).Build(),
			wantManagedCertificate: managedcertificate.
				New(types.NewId("default", "foo"), "example.com").
				WithStatus("Active", "Active").
				WithCertificateName("foo").
				Build(),
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
			ssl: ssl.NewFake().AddEntryWithStatus("foo", "ACTIVE",
				map[string]string{"example.com": "ACTIVE"}).Build(),
			random: random.NewFake("foo"),

			wantState: state.NewFakeWithEntries(map[types.Id]state.Entry{
				types.NewId("default", "foo"): state.Entry{
					SslCertificateName:             "foo",
					SslCertificateCreationReported: true,
				},
			}),
			wantSsl: ssl.NewFake().AddEntryWithStatus("foo", "ACTIVE",
				map[string]string{"example.com": "ACTIVE"}).Build(),
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
			ssl:    ssl.NewFake().Build(),
			random: random.NewFake("foo"),

			wantState: state.NewFakeWithEntries(map[types.Id]state.Entry{
				types.NewId("default", "foo"): state.Entry{
					SslCertificateName:             "foo",
					SoftDeleted:                    false,
					SslCertificateCreationReported: true,
				},
			}),
			wantSsl: ssl.NewFake().AddEntry("foo", []string{"example.com"}).Build(),
			wantManagedCertificate: managedcertificate.
				New(types.NewId("default", "foo"), "example.com").
				WithStatus("", "").
				WithCertificateName("foo").
				Build(),
			wantMetrics: &metrics.Fake{CreationCnt: 1},
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
			ssl:    ssl.NewFake().Build(),
			random: random.NewFake("foo"),

			wantState: state.NewFakeWithEntries(map[types.Id]state.Entry{
				types.NewId("default", "foo"): state.Entry{
					SslCertificateName:             "foo",
					SslCertificateCreationReported: true,
				},
			}),
			wantSsl: ssl.NewFake().AddEntry("foo", []string{"example.com"}).Build(),
			wantManagedCertificate: managedcertificate.
				New(types.NewId("default", "foo"), "example.com").
				WithCertificateName("foo").
				WithStatus("", "").
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
			ssl:    ssl.NewFake().Build(),
			random: random.NewFake("foo"),

			wantState: state.NewFakeWithEntries(map[types.Id]state.Entry{
				types.NewId("default", "foo"): state.Entry{
					SslCertificateName: "foo",
					ExcludedFromSLO:    true,
				},
			}),
			wantSsl: ssl.NewFake().AddEntry("foo", []string{"example.com"}).Build(),
			wantManagedCertificate: managedcertificate.
				New(types.NewId("default", "foo"), "example.com").
				WithCertificateName("foo").
				WithStatus("", "").
				Build(),
			wantMetrics: metrics.NewFake(),
		},
		"API server: found, state: found not reported, GCP: not found": {
			managedCertificate: managedcertificate.
				New(types.NewId("default", "foo"), "example.com").Build(),
			state: state.NewFakeWithEntries(map[types.Id]state.Entry{
				types.NewId("default", "foo"): state.Entry{SslCertificateName: "foo"},
			}),
			ssl:    ssl.NewFake().Build(),
			random: random.NewFake("foo"),

			wantState: state.NewFakeWithEntries(map[types.Id]state.Entry{
				types.NewId("default", "foo"): state.Entry{
					SslCertificateName:             "foo",
					SslCertificateCreationReported: true,
				},
			}),
			wantSsl: ssl.NewFake().AddEntry("foo", []string{"example.com"}).Build(),
			wantManagedCertificate: managedcertificate.
				New(types.NewId("default", "foo"), "example.com").
				WithCertificateName("foo").
				WithStatus("", "").
				Build(),
			wantMetrics: &metrics.Fake{CreationCnt: 1},
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
			ssl: ssl.NewFake().AddEntryWithStatus("foo", "ACTIVE",
				map[string]string{"different-domain.com": "ACTIVE"}).Build(),
			random: random.NewFake("foo"),

			wantState: state.NewFakeWithEntries(map[types.Id]state.Entry{
				types.NewId("default", "foo"): state.Entry{
					SslCertificateName:             "foo",
					SslCertificateCreationReported: true,
				},
			}),
			wantSsl: ssl.NewFake().AddEntry("foo", []string{"example.com"}).Build(),
			wantManagedCertificate: managedcertificate.
				New(types.NewId("default", "foo"), "example.com").
				WithCertificateName("foo").
				WithStatus("", "").
				Build(),
			wantMetrics: metrics.NewFake(),
		},
		"API server: found, state: found soft-deleted, GCP: found; certificates different": {
			managedCertificate: managedcertificate.
				New(types.NewId("default", "foo"), "example.com").Build(),
			state: state.NewFakeWithEntries(map[types.Id]state.Entry{
				types.NewId("default", "foo"): state.Entry{
					SslCertificateName:             "foo",
					SslCertificateCreationReported: true,
					SoftDeleted:                    true,
				},
			}),
			ssl: ssl.NewFake().AddEntryWithStatus("foo", "ACTIVE",
				map[string]string{"different-domain.com": "ACTIVE"}).Build(),
			random: random.NewFake("bar"),

			wantState: state.NewFakeWithEntries(map[types.Id]state.Entry{
				types.NewId("default", "foo"): state.Entry{
					SslCertificateName:             "foo",
					SslCertificateCreationReported: true,
					SoftDeleted:                    false,
				},
			}),
			wantSsl: ssl.NewFake().AddEntry("foo", []string{"example.com"}).Build(),
			wantManagedCertificate: managedcertificate.
				New(types.NewId("default", "foo"), "example.com").
				WithCertificateName("foo").
				WithStatus("", "").
				Build(),
			wantMetrics: metrics.NewFake(),
		},
	} {
		testCase := testCase
		t.Run(description, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()

			var managedCertificates []*v1.ManagedCertificate
			if testCase.managedCertificate != nil {
				managedCertificates = append(managedCertificates,
					testCase.managedCertificate)
			}

			managedCertificate := clientsmcrt.NewFake(managedCertificates)
			event := &event.Fake{}
			metrics := metrics.NewFake()

			sync := New(config.NewFake(), event, clientsingress.NewFake(nil),
				managedCertificate, metrics, testCase.random,
				sslcertificatemanager.New(event, metrics, testCase.ssl, testCase.state),
				testCase.state)

			id := types.NewId("default", "foo")
			if err := sync.ManagedCertificate(ctx, id); err != testCase.wantError {
				t.Fatalf("sync.ManagedCertificate(%s): %v, want %v",
					id, err, testCase.wantError)
			}

			if diff := cmp.Diff(testCase.wantState.List(), testCase.state.List()); diff != "" {
				t.Fatalf("Diff state (-want, +got): %s", diff)
			}

			wantSslCert, wantSslCertErr := getSslCert(id, testCase.wantState, testCase.wantSsl)
			gotSslCert, gotSslCertErr := getSslCert(id, testCase.state, testCase.ssl)
			sslCertDiff := cmp.Diff(wantSslCert, gotSslCert)
			if wantSslCertErr != gotSslCertErr || sslCertDiff != "" {
				t.Fatalf(`Diff SslCertificate (-want, +got): %s,
					got error: %v, want error: %v`,
					sslCertDiff, gotSslCertErr, wantSslCertErr)
			}

			if testCase.wantManagedCertificate != nil && len(managedCertificates) != 1 {
				t.Fatalf(`ManagedCertificate nil, want %+v;
					total number of certificates: %d, want 1`,
					testCase.wantManagedCertificate, len(managedCertificates))
			} else if testCase.wantManagedCertificate == nil &&
				len(managedCertificates) != 0 {

				t.Fatalf(`ManagedCertificate %+v, want nil;
					total number of certificates: %d, want 0`,
					managedCertificates[0], len(managedCertificates))
			} else if len(managedCertificates) > 0 {
				if diff := cmp.Diff(testCase.wantManagedCertificate,
					managedCertificates[0]); diff != "" {

					t.Fatalf("Diff ManagedCertificates (-want, +got): %s", diff)
				}
			}

			if diff := cmp.Diff(testCase.wantMetrics, metrics); diff != "" {
				t.Fatalf("Diff metrics (-want, +got): %s", diff)
			}
		})
	}
}
