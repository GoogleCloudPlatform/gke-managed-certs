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
	"testing"

	"google.golang.org/api/googleapi"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	cgotesting "k8s.io/client-go/testing"

	apisv1beta2 "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/networking.gke.io/v1beta2"
	clientsetv1beta2 "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/clientset/versioned/typed/networking.gke.io/v1beta2/fake"
	listersv1beta2 "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/listers/networking.gke.io/v1beta2"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/config"
	cnterrors "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/errors"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/fake"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/types"
)

const (
	domainBar          = "bar.com"
	domainFoo          = "foo.com"
	sslCertificateName = "baz"
)

var (
	mcrtId = types.NewCertId("foo", "bar")
)

func buildUpdateFunc(updateCalled *bool) cgotesting.ReactionFunc {
	return cgotesting.ReactionFunc(func(action cgotesting.Action) (bool, runtime.Object, error) {
		*updateCalled = true
		return true, nil, nil
	})
}

var genericError = errors.New("generic error")
var googleNotFound = &googleapi.Error{
	Code: 404,
}
var k8sNotFound = k8serrors.NewNotFound(schema.GroupResource{
	Group:    "test_group",
	Resource: "test_resource",
}, "test_name")

func mockMcrt(domain string) *apisv1beta2.ManagedCertificate {
	return fake.NewManagedCertificate(mcrtId, domain)
}

var listerFailsGenericErr = fake.NewLister(genericError, nil)
var listerFailsNotFound = fake.NewLister(k8sNotFound, nil)
var listerSuccess = fake.NewLister(nil, []*apisv1beta2.ManagedCertificate{mockMcrt(domainFoo)})

var randomFailsGenericErr = newRandom(genericError, "")
var randomSuccess = newRandom(nil, sslCertificateName)

func empty() *fake.FakeState {
	return fake.NewState()
}
func withEntry() *fake.FakeState {
	return fake.NewStateWithEntries(map[types.CertId]fake.StateEntry{
		mcrtId: fake.StateEntry{SslCertificateName: sslCertificateName},
	})
}
func withEntryAndExcludedFromSLOFails() *fake.FakeState {
	return fake.NewStateWithEntries(map[types.CertId]fake.StateEntry{
		mcrtId: fake.StateEntry{
			SslCertificateName: sslCertificateName,
			ExcludedFromSLOErr: genericError,
		},
	})
}
func withEntryAndExcludedFromSLOSet() *fake.FakeState {
	return fake.NewStateWithEntries(map[types.CertId]fake.StateEntry{
		mcrtId: fake.StateEntry{
			SslCertificateName: sslCertificateName,
			ExcludedFromSLO:    true,
		},
	})
}
func withEntryAndSslCertificateCreationFails() *fake.FakeState {
	return fake.NewStateWithEntries(map[types.CertId]fake.StateEntry{
		mcrtId: fake.StateEntry{
			SslCertificateName:        sslCertificateName,
			SslCertificateCreationErr: cnterrors.ErrManagedCertificateNotFound,
		},
	})
}
func withEntryAndSslCertificateCreationReported() *fake.FakeState {
	return fake.NewStateWithEntries(map[types.CertId]fake.StateEntry{
		mcrtId: fake.StateEntry{
			SslCertificateName:             sslCertificateName,
			SslCertificateCreationReported: true,
		},
	})
}
func withEntryAndSoftDeleted() *fake.FakeState {
	return fake.NewStateWithEntries(map[types.CertId]fake.StateEntry{
		mcrtId: fake.StateEntry{
			SslCertificateName: sslCertificateName,
			SoftDeleted:        true,
		},
	})
}
func withEntryAndSoftDeletedFails() *fake.FakeState {
	return fake.NewStateWithEntries(map[types.CertId]fake.StateEntry{
		mcrtId: fake.StateEntry{
			SslCertificateName: sslCertificateName,
			SoftDeletedErr:     cnterrors.ErrManagedCertificateNotFound,
		},
	})
}

type in struct {
	lister       listersv1beta2.ManagedCertificateLister
	metrics      *fake.FakeMetrics
	random       fakeRandom
	state        *fake.FakeState
	mcrt         *apisv1beta2.ManagedCertificate
	sslCreateErr error
	sslDeleteErr error
	sslExistsErr error
	sslGetErr    error
}

type out struct {
	entryInState                bool
	createLatencyMetricObserved bool
	wantSoftDeleted             bool
	wantEcludedFromSLO          bool
	wantUpdateCalled            bool
	err                         error
}

var testCases = []struct {
	desc string
	in   in
	out  out
}{
	{
		"Lister fails with generic error, state is empty",
		in{
			lister:  listerFailsGenericErr,
			metrics: fake.NewMetrics(),
			random:  randomSuccess,
			state:   empty(),
		},
		out{
			entryInState:     false,
			wantUpdateCalled: false,
			err:              genericError,
		},
	},
	{
		"Lister fails with generic error, entry in state",
		in{
			lister:  listerFailsGenericErr,
			metrics: fake.NewMetrics(),
			random:  randomSuccess,
			state:   withEntry(),
		},
		out{
			entryInState:     true,
			wantUpdateCalled: false,
			err:              genericError,
		},
	},
	{
		"Lister fails with not found, state is empty",
		in{
			lister:  listerFailsNotFound,
			metrics: fake.NewMetrics(),
			random:  randomSuccess,
			state:   empty(),
		},
		out{
			entryInState:     false,
			wantUpdateCalled: false,
			err:              nil,
		},
	},
	{
		"Lister fails with not found, entry in state, soft deleted in state, success to delete SslCertificate",
		in{
			lister:  listerFailsNotFound,
			metrics: fake.NewMetrics(),
			random:  randomSuccess,
			state:   withEntryAndSoftDeleted(),
		},
		out{
			entryInState:     false,
			wantSoftDeleted:  true,
			wantUpdateCalled: false,
			err:              nil,
		},
	},
	{
		"Lister fails with not found, entry in state, setting soft deleted fails",
		in{
			lister:  listerFailsNotFound,
			metrics: fake.NewMetrics(),
			random:  randomSuccess,
			state:   withEntryAndSoftDeletedFails(),
		},
		out{
			entryInState:     true,
			wantUpdateCalled: false,
			err:              cnterrors.ErrManagedCertificateNotFound,
		},
	},
	{
		"Lister fails with not found, entry in state, success to delete SslCertificate",
		in{
			lister:  listerFailsNotFound,
			metrics: fake.NewMetrics(),
			random:  randomSuccess,
			state:   withEntry(),
		},
		out{
			entryInState:     false,
			wantSoftDeleted:  true,
			wantUpdateCalled: false,
			err:              nil,
		},
	},
	{
		"Lister fails with not found, entry in state, SslCertificate already deleted",
		in{
			lister:       listerFailsNotFound,
			metrics:      fake.NewMetrics(),
			random:       randomSuccess,
			state:        withEntry(),
			sslDeleteErr: googleNotFound,
		},
		out{
			entryInState:     false,
			wantSoftDeleted:  true,
			wantUpdateCalled: false,
			err:              nil,
		},
	},
	{
		"Lister fails with not found, entry in state, fail to delete SslCertificate",
		in{
			lister:       listerFailsNotFound,
			metrics:      fake.NewMetrics(),
			random:       randomSuccess,
			state:        withEntry(),
			sslDeleteErr: genericError,
		},
		out{
			entryInState:     true,
			wantSoftDeleted:  true,
			wantUpdateCalled: false,
			err:              genericError,
		},
	},
	{
		"Lister success, state empty",
		in{
			lister:  listerSuccess,
			metrics: fake.NewMetrics(),
			random:  randomSuccess,
			state:   empty(),
		},
		out{
			entryInState:                true,
			createLatencyMetricObserved: true,
			wantUpdateCalled:            true,
			err:                         nil,
		},
	},
	{
		"Lister success, entry in state",
		in{
			lister:  listerSuccess,
			metrics: fake.NewMetrics(),
			random:  randomSuccess,
			state:   withEntry(),
		},
		out{
			entryInState:                true,
			createLatencyMetricObserved: true,
			wantUpdateCalled:            true,
			err:                         nil,
		},
	},
	{
		"Lister success, state empty, random fails",
		in{
			lister:  listerSuccess,
			metrics: fake.NewMetrics(),
			random:  randomFailsGenericErr,
			state:   empty(),
		},
		out{
			entryInState:     false,
			wantUpdateCalled: false,
			err:              genericError,
		},
	},
	{
		"Lister success, entry in state, random fails",
		in{
			lister:  listerSuccess,
			metrics: fake.NewMetrics(),
			random:  randomFailsGenericErr,
			state:   withEntry(),
		},
		out{
			entryInState:                true,
			createLatencyMetricObserved: true,
			wantUpdateCalled:            true,
			err:                         nil,
		},
	},
	{
		"Lister success, state empty, SslCertificate exists fails",
		in{
			lister:       listerSuccess,
			metrics:      fake.NewMetrics(),
			random:       randomSuccess,
			state:        empty(),
			sslExistsErr: genericError,
		},
		out{
			entryInState:     true,
			wantUpdateCalled: false,
			err:              genericError,
		},
	},
	{
		"Lister success, entry in state, SslCertificate exists fails",
		in{
			lister:       listerSuccess,
			metrics:      fake.NewMetrics(),
			random:       randomSuccess,
			state:        withEntry(),
			sslExistsErr: genericError,
		},
		out{
			entryInState:     true,
			wantUpdateCalled: false,
			err:              genericError,
		},
	},
	{
		"Lister success, state empty, SslCertificate creation fails",
		in{
			lister:       listerSuccess,
			metrics:      fake.NewMetrics(),
			random:       randomSuccess,
			state:        empty(),
			sslCreateErr: genericError,
		},
		out{
			entryInState:     true,
			wantUpdateCalled: false,
			err:              genericError,
		},
	},
	{
		"Lister success, entry in state, SslCertificate creation fails",
		in{
			lister:       listerSuccess,
			metrics:      fake.NewMetrics(),
			random:       randomSuccess,
			state:        withEntry(),
			sslCreateErr: genericError,
		},
		out{
			entryInState:     true,
			wantUpdateCalled: false,
			err:              genericError,
		},
	},
	{
		"Lister success, entry in state, SslCertificate creation succeeds, excluded from SLO: entry not found",
		in{
			lister:  listerSuccess,
			metrics: fake.NewMetrics(),
			random:  randomSuccess,
			state:   withEntryAndExcludedFromSLOFails(),
		},
		out{
			entryInState:     true,
			wantUpdateCalled: false,
			err:              genericError,
		},
	},
	{
		"Lister success, entry in state, SslCertificate creation succeeds, excluded from SLO",
		in{
			lister:  listerSuccess,
			metrics: fake.NewMetrics(),
			random:  randomSuccess,
			state:   withEntryAndExcludedFromSLOSet(),
		},
		out{
			entryInState:                true,
			createLatencyMetricObserved: false,
			wantUpdateCalled:            true,
			err:                         nil,
		},
	},
	{
		"Lister success, entry in state, SslCertificate creation succeeds, metric reported entry not found",
		in{
			lister:  listerSuccess,
			metrics: fake.NewMetrics(),
			random:  randomSuccess,
			state:   withEntryAndSslCertificateCreationFails(),
		},
		out{
			entryInState:     true,
			wantUpdateCalled: false,
			err:              cnterrors.ErrManagedCertificateNotFound,
		},
	},
	{
		"Lister success, entry in state, SslCertificate creation succeeds, metric already reported",
		in{
			lister:  listerSuccess,
			metrics: fake.NewMetricsSslCertificateCreationAlreadyReported(),
			random:  randomSuccess,
			state:   withEntryAndSslCertificateCreationReported(),
		},
		out{
			entryInState:                true,
			createLatencyMetricObserved: true,
			wantUpdateCalled:            true,
			err:                         nil,
		},
	},
	{
		"Lister success, state empty, SslCertificate does not exists - get fails",
		in{
			lister:    listerSuccess,
			metrics:   fake.NewMetrics(),
			random:    randomSuccess,
			state:     empty(),
			sslGetErr: genericError,
		},
		out{
			entryInState:                true,
			createLatencyMetricObserved: true,
			wantUpdateCalled:            false,
			err:                         genericError,
		},
	},
	{
		"Lister success, entry in state, SslCertificate does not exists - get fails",
		in{
			lister:    listerSuccess,
			metrics:   fake.NewMetrics(),
			random:    randomSuccess,
			state:     withEntry(),
			sslGetErr: genericError,
		},
		out{
			entryInState:                true,
			createLatencyMetricObserved: true,
			wantUpdateCalled:            false,
			err:                         genericError,
		},
	},
	{
		"Lister success, state empty, SslCertificate exists - get fails",
		in{
			lister:    listerSuccess,
			metrics:   fake.NewMetrics(),
			random:    randomSuccess,
			state:     empty(),
			mcrt:      mockMcrt(domainFoo),
			sslGetErr: genericError,
		},
		out{
			entryInState:     true,
			wantUpdateCalled: false,
			err:              genericError,
		},
	},
	{
		"Lister success, entry in state, SslCertificate exists - get fails",
		in{
			lister:    listerSuccess,
			metrics:   fake.NewMetrics(),
			random:    randomSuccess,
			state:     withEntry(),
			mcrt:      mockMcrt(domainFoo),
			sslGetErr: genericError,
		},
		out{
			entryInState:     true,
			wantUpdateCalled: false,
			err:              genericError,
		},
	},
	{
		"Lister success, entry in state soft deleted, SslCertificate exists",
		in{
			lister:  listerSuccess,
			metrics: fake.NewMetrics(),
			random:  randomSuccess,
			state:   withEntryAndSoftDeleted(),
			mcrt:    mockMcrt(domainFoo),
		},
		out{
			entryInState:     false,
			wantUpdateCalled: false,
		},
	},
	{
		"Lister success, entry in state, certs mismatch",
		in{
			lister:  listerSuccess,
			metrics: fake.NewMetrics(),
			random:  randomSuccess,
			state:   withEntry(),
			mcrt:    mockMcrt(domainBar),
		},
		out{
			entryInState:     false,
			wantSoftDeleted:  true,
			wantUpdateCalled: false,
			err:              cnterrors.ErrSslCertificateOutOfSyncGotDeleted,
		},
	},
	{
		"Lister success, entry in state, certs mismatch - SslCertificate already deleted",
		in{
			lister:       listerSuccess,
			metrics:      fake.NewMetrics(),
			random:       randomSuccess,
			state:        withEntry(),
			mcrt:         mockMcrt(domainBar),
			sslDeleteErr: googleNotFound,
		},
		out{
			entryInState:     false,
			wantSoftDeleted:  true,
			wantUpdateCalled: false,
			err:              cnterrors.ErrSslCertificateOutOfSyncGotDeleted,
		},
	},
	{
		"Lister success, entry in state, certs mismatch - SslCertificate deletion fails",
		in{
			lister:       listerSuccess,
			metrics:      fake.NewMetrics(),
			random:       randomSuccess,
			state:        withEntry(),
			mcrt:         mockMcrt(domainBar),
			sslDeleteErr: genericError,
		},
		out{
			entryInState:     true,
			wantSoftDeleted:  true,
			wantUpdateCalled: false,
			err:              genericError,
		},
	},
	{
		"Lister success, entry in state, certs mismatch, setting soft deleted fails",
		in{
			lister:  listerSuccess,
			metrics: fake.NewMetrics(),
			random:  randomSuccess,
			state:   withEntryAndSoftDeletedFails(),
			mcrt:    mockMcrt(domainBar),
		},
		out{
			entryInState:     true,
			wantUpdateCalled: false,
			err:              cnterrors.ErrManagedCertificateNotFound,
		},
	},
}

func TestManagedCertificate(t *testing.T) {
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			ctx := context.Background()

			client := &clientsetv1beta2.FakeNetworkingV1beta2{Fake: &cgotesting.Fake{}}
			updateCalled := false
			client.AddReactor("update", "*", buildUpdateFunc(&updateCalled))

			config := config.NewFakeCertificateStatusConfig()
			ssl := newSsl(sslCertificateName, tc.in.mcrt, tc.in.sslCreateErr, tc.in.sslDeleteErr,
				tc.in.sslExistsErr, tc.in.sslGetErr)
			sut := New(client, config, tc.in.lister, tc.in.metrics, tc.in.random, ssl, tc.in.state)

			if err := sut.ManagedCertificate(ctx, mcrtId); err != tc.out.err {
				t.Errorf("Have error: %v, want: %v", err, tc.out.err)
			}

			if _, err := tc.in.state.GetSslCertificateName(mcrtId); (err == nil) != tc.out.entryInState {
				t.Errorf("Entry in state %t, want %t, err: %v", err == nil, tc.out.entryInState, err)
			}

			softDeleted, err := tc.in.state.IsSoftDeleted(mcrtId)
			if err == nil && tc.out.wantSoftDeleted != softDeleted {
				t.Errorf("Soft deleted: %t, want: %t", softDeleted, tc.out.wantSoftDeleted)
			}

			reported, _ := tc.in.state.IsSslCertificateCreationReported(mcrtId)
			_, err = tc.in.state.GetSslCertificateName(mcrtId)
			entryExists := err == nil
			if entryExists != tc.out.entryInState || reported != tc.out.createLatencyMetricObserved {
				t.Errorf("Entry in state %t, want %t; CreateSslCertificateLatency metric observed %t, want %t",
					entryExists, tc.out.entryInState, reported, tc.out.createLatencyMetricObserved)
			}
			if tc.out.createLatencyMetricObserved && tc.in.metrics.SslCertificateCreationLatencyObserved != 1 {
				t.Errorf("CreateSslCertificateLatency metric observed %d times, want 1",
					tc.in.metrics.SslCertificateCreationLatencyObserved)
			}

			if tc.out.wantUpdateCalled != updateCalled {
				t.Errorf("Update called %t, want %t", updateCalled, tc.out.wantUpdateCalled)
			}
		})
	}
}
