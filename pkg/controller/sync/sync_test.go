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
	"testing"

	"github.com/google/go-cmp/cmp"
	compute "google.golang.org/api/compute/v1"

	apisv1 "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/networking.gke.io/v1"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/config"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/errors"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/metrics"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/sslcertificatemanager"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/state"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/testhelper/managedcertificate"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/random"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/types"
)

var testCases = map[string]struct {
	managedCertificate *apisv1.ManagedCertificate
	state              state.State
	sslManager         sslcertificatemanager.SslCertificateManager
	random             random.Random

	wantState              state.State
	wantSslManager         sslcertificatemanager.SslCertificateManager
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
		state: state.NewFakeWithEntries(map[types.CertId]state.Entry{
			types.NewCertId("default", "foo"): state.Entry{SslCertificateName: "foo"},
		}),
		sslManager: sslcertificatemanager.NewFake(),
		random:     random.New(""),

		wantState:      state.NewFake(),
		wantSslManager: sslcertificatemanager.NewFake(),
		wantMetrics:    metrics.NewFake(),
	},
	"API server: not found, state: found, GCP: found": {
		state: state.NewFakeWithEntries(map[types.CertId]state.Entry{
			types.NewCertId("default", "foo"): state.Entry{
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
		state: state.NewFakeWithEntries(map[types.CertId]state.Entry{
			types.NewCertId("default", "foo"): state.Entry{
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
			New(types.NewCertId("default", "foo"), "example.com").Build(),
		state:      state.NewFake(),
		sslManager: sslcertificatemanager.NewFake(),
		random:     random.NewFake("foo", nil),

		wantState: state.NewFakeWithEntries(map[types.CertId]state.Entry{
			types.NewCertId("default", "foo"): state.Entry{
				SslCertificateName:             "foo",
				SslCertificateCreationReported: true,
			},
		}),
		wantSslManager: sslcertificatemanager.
			NewFakeWithEntry("foo", []string{"example.com"}, "", nil),
		wantManagedCertificate: managedcertificate.
			New(types.NewCertId("default", "foo"), "example.com").
			WithCertificateName("foo").
			Build(),
		wantMetrics: &metrics.Fake{SslCertificateCreationLatencyObserved: 1},
	},
	"API server: found, state: not found, GCP: found": {
		managedCertificate: managedcertificate.
			New(types.NewCertId("default", "foo"), "example.com").Build(),
		state: state.NewFake(),
		sslManager: sslcertificatemanager.
			NewFakeWithEntry("foo", []string{"example.com"},
				"ACTIVE", []string{"ACTIVE"}),
		random: random.NewFake("foo", nil),

		wantState: state.NewFakeWithEntries(map[types.CertId]state.Entry{
			types.NewCertId("default", "foo"): state.Entry{SslCertificateName: "foo"},
		}),
		wantSslManager: sslcertificatemanager.
			NewFakeWithEntry("foo", []string{"example.com"},
				"ACTIVE", []string{"ACTIVE"}),
		wantManagedCertificate: managedcertificate.
			New(types.NewCertId("default", "foo"), "example.com").
			WithStatus("Active", "Active").
			WithCertificateName("foo").
			Build(),
		wantMetrics: metrics.NewFake(),
	},
	"API server: found, state: found soft deleted, GCP: found": {
		managedCertificate: managedcertificate.
			New(types.NewCertId("default", "foo"), "example.com").Build(),
		state: state.NewFakeWithEntries(map[types.CertId]state.Entry{
			types.NewCertId("default", "foo"): state.Entry{
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
			New(types.NewCertId("default", "foo"), "example.com").Build(),
		wantMetrics: metrics.NewFake(),
	},
	"API server: found, state: found, GCP: found": {
		managedCertificate: managedcertificate.
			New(types.NewCertId("default", "foo"), "example.com").Build(),
		state: state.NewFakeWithEntries(map[types.CertId]state.Entry{
			types.NewCertId("default", "foo"): state.Entry{
				SslCertificateName:             "foo",
				SslCertificateCreationReported: true,
			},
		}),
		sslManager: sslcertificatemanager.
			NewFakeWithEntry("foo", []string{"example.com"},
				"ACTIVE", []string{"ACTIVE"}),
		random: random.NewFake("foo", nil),

		wantState: state.NewFakeWithEntries(map[types.CertId]state.Entry{
			types.NewCertId("default", "foo"): state.Entry{
				SslCertificateName:             "foo",
				SslCertificateCreationReported: true,
			},
		}),
		wantSslManager: sslcertificatemanager.
			NewFakeWithEntry("foo", []string{"example.com"},
				"ACTIVE", []string{"ACTIVE"}),
		wantManagedCertificate: managedcertificate.
			New(types.NewCertId("default", "foo"), "example.com").
			WithStatus("Active", "Active").
			WithCertificateName("foo").
			Build(),
		wantMetrics: metrics.NewFake(),
	},
	"API server: found, state: found soft deleted, GCP: not found": {
		managedCertificate: managedcertificate.
			New(types.NewCertId("default", "foo"), "example.com").Build(),
		state: state.NewFakeWithEntries(map[types.CertId]state.Entry{
			types.NewCertId("default", "foo"): state.Entry{
				SslCertificateName: "foo",
				SoftDeleted:        true,
			},
		}),
		sslManager: sslcertificatemanager.NewFake(),
		random:     random.NewFake("foo", nil),

		wantState:      state.NewFake(),
		wantSslManager: sslcertificatemanager.NewFake(),
		wantManagedCertificate: managedcertificate.
			New(types.NewCertId("default", "foo"), "example.com").Build(),
		wantMetrics: metrics.NewFake(),
	},
	"API server: found, state: found reported, GCP: not found": {
		managedCertificate: managedcertificate.
			New(types.NewCertId("default", "foo"), "example.com").Build(),
		state: state.NewFakeWithEntries(map[types.CertId]state.Entry{
			types.NewCertId("default", "foo"): state.Entry{
				SslCertificateName:             "foo",
				SslCertificateCreationReported: true,
			},
		}),
		sslManager: sslcertificatemanager.NewFake(),
		random:     random.NewFake("foo", nil),

		wantState: state.NewFakeWithEntries(map[types.CertId]state.Entry{
			types.NewCertId("default", "foo"): state.Entry{
				SslCertificateName:             "foo",
				SslCertificateCreationReported: true,
			},
		}),
		wantSslManager: sslcertificatemanager.
			NewFakeWithEntry("foo", []string{"example.com"}, "", nil),
		wantManagedCertificate: managedcertificate.
			New(types.NewCertId("default", "foo"), "example.com").
			WithCertificateName("foo").
			Build(),
		wantMetrics: metrics.NewFake(),
	},
	"API server: found, state: found excluded from SLO, GCP: not found": {
		managedCertificate: managedcertificate.
			New(types.NewCertId("default", "foo"), "example.com").Build(),
		state: state.NewFakeWithEntries(map[types.CertId]state.Entry{
			types.NewCertId("default", "foo"): state.Entry{
				SslCertificateName: "foo",
				ExcludedFromSLO:    true,
			},
		}),
		sslManager: sslcertificatemanager.NewFake(),
		random:     random.NewFake("foo", nil),

		wantState: state.NewFakeWithEntries(map[types.CertId]state.Entry{
			types.NewCertId("default", "foo"): state.Entry{
				SslCertificateName: "foo",
				ExcludedFromSLO:    true,
			},
		}),
		wantSslManager: sslcertificatemanager.
			NewFakeWithEntry("foo", []string{"example.com"}, "", nil),
		wantManagedCertificate: managedcertificate.
			New(types.NewCertId("default", "foo"), "example.com").
			WithCertificateName("foo").
			Build(),
		wantMetrics: metrics.NewFake(),
	},
	"API server: found, state: found not reported, GCP: not found": {
		managedCertificate: managedcertificate.
			New(types.NewCertId("default", "foo"), "example.com").Build(),
		state: state.NewFakeWithEntries(map[types.CertId]state.Entry{
			types.NewCertId("default", "foo"): state.Entry{SslCertificateName: "foo"},
		}),
		sslManager: sslcertificatemanager.NewFake(),
		random:     random.NewFake("foo", nil),

		wantState: state.NewFakeWithEntries(map[types.CertId]state.Entry{
			types.NewCertId("default", "foo"): state.Entry{
				SslCertificateName:             "foo",
				SslCertificateCreationReported: true,
			},
		}),
		wantSslManager: sslcertificatemanager.
			NewFakeWithEntry("foo", []string{"example.com"}, "", nil),
		wantManagedCertificate: managedcertificate.
			New(types.NewCertId("default", "foo"), "example.com").
			WithCertificateName("foo").
			Build(),
		wantMetrics: &metrics.Fake{SslCertificateCreationLatencyObserved: 1},
	},
	"API server: found, state: found, GCP: found; certificates different": {
		managedCertificate: managedcertificate.
			New(types.NewCertId("default", "foo"), "example.com").Build(),
		state: state.NewFakeWithEntries(map[types.CertId]state.Entry{
			types.NewCertId("default", "foo"): state.Entry{
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
			New(types.NewCertId("default", "foo"), "example.com").
			Build(),
		wantMetrics: metrics.NewFake(),
		wantError:   errors.ErrSslCertificateOutOfSyncGotDeleted,
	},
}

func getSslCertificate(id types.CertId, state state.State,
	sslManager sslcertificatemanager.SslCertificateManager) (*compute.SslCertificate, error) {

	entry, err := state.Get(id)
	if err != nil {
		return nil, err
	}

	return sslManager.Get(entry.SslCertificateName, nil)
}

func TestManagedCertificate(t *testing.T) {
	for description, tc := range testCases {
		t.Run(description, func(t *testing.T) {
			ctx := context.Background()

			var managedCertificates []*apisv1.ManagedCertificate
			if tc.managedCertificate != nil {
				managedCertificates = append(managedCertificates,
					tc.managedCertificate)
			}

			clientset := managedcertificate.NewClientset(managedCertificates).
				NetworkingV1()
			lister := managedcertificate.NewLister(managedCertificates)
			config := config.NewFakeCertificateStatusConfig()
			metrics := metrics.NewFake()
			sync := New(clientset, config, lister, metrics, tc.random,
				tc.sslManager, tc.state)

			id := types.NewCertId("default", "foo")
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
