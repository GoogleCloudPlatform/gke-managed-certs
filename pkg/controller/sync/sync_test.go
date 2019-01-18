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
	"testing"

	"google.golang.org/api/googleapi"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	cgo_testing "k8s.io/client-go/testing"

	api "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/networking.gke.io/v1beta1"
	fakenetworkingv1beta1 "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/clientset/versioned/typed/networking.gke.io/v1beta1/fake"
	mcrtlister "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/listers/networking.gke.io/v1beta1"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/config"
	cnt_errors "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/errors"
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

func buildUpdateFunc(updateCalled *bool) cgo_testing.ReactionFunc {
	return cgo_testing.ReactionFunc(func(action cgo_testing.Action) (bool, runtime.Object, error) {
		*updateCalled = true
		return true, nil, nil
	})
}

var genericError = errors.New("generic error")
var googleNotFound = &googleapi.Error{
	Code: 404,
}
var k8sNotFound = k8s_errors.NewNotFound(schema.GroupResource{
	Group:    "test_group",
	Resource: "test_resource",
}, "test_name")

func mockMcrt(domain string) *api.ManagedCertificate {
	return &api.ManagedCertificate{
		ObjectMeta: metav1.ObjectMeta{
			CreationTimestamp: metav1.Now().Rfc3339Copy(),
			Namespace:         mcrtId.Namespace,
			Name:              mcrtId.Name,
		},
		Spec: api.ManagedCertificateSpec{
			Domains: []string{domain},
		},
	}
}

var listerFailsGenericErr = fake.NewLister(genericError, nil)
var listerFailsNotFound = fake.NewLister(k8sNotFound, nil)
var listerSuccess = fake.NewLister(nil, []*api.ManagedCertificate{mockMcrt(domainFoo)})

var randomFailsGenericErr = newRandom(genericError, "")
var randomSuccess = newRandom(nil, sslCertificateName)

func empty() *fakeState {
	return newEmptyState()
}
func withEntry() *fakeState {
	return newState(mcrtId, sslCertificateName)
}
func withEntryAndSslCertificateCreationFails() *fakeState {
	return newStateWithOverride(mcrtId, sslCertificateName, false, cnt_errors.ErrManagedCertificateNotFound, false, nil)
}
func withEntryAndSslCertificateCreationReported() *fakeState {
	return newStateWithOverride(mcrtId, sslCertificateName, true, nil, false, nil)
}
func withEntryAndSoftDeletedFails() *fakeState {
	return newStateWithOverride(mcrtId, sslCertificateName, false, nil, false, cnt_errors.ErrManagedCertificateNotFound)
}
func withEntryAndSoftDeleted() *fakeState {
	return newStateWithOverride(mcrtId, sslCertificateName, false, nil, true, nil)
}

type in struct {
	lister       mcrtlister.ManagedCertificateLister
	metrics      *fake.FakeMetrics
	random       fakeRandom
	state        *fakeState
	mcrt         *api.ManagedCertificate
	sslCreateErr error
	sslDeleteErr error
	sslExistsErr error
	sslGetErr    error
}

type out struct {
	entryInState                bool
	createLatencyMetricObserved bool
	wantSoftDeleted             bool
	updateCalled                bool
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
			entryInState: false,
			updateCalled: false,
			err:          genericError,
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
			entryInState: true,
			updateCalled: false,
			err:          genericError,
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
			entryInState: false,
			updateCalled: false,
			err:          nil,
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
			entryInState:    false,
			wantSoftDeleted: true,
			updateCalled:    false,
			err:             nil,
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
			entryInState: true,
			updateCalled: false,
			err:          cnt_errors.ErrManagedCertificateNotFound,
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
			entryInState:    false,
			wantSoftDeleted: true,
			updateCalled:    false,
			err:             nil,
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
			entryInState:    false,
			wantSoftDeleted: true,
			updateCalled:    false,
			err:             nil,
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
			entryInState:    true,
			wantSoftDeleted: true,
			updateCalled:    false,
			err:             genericError,
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
			updateCalled:                true,
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
			updateCalled:                true,
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
			entryInState: false,
			updateCalled: false,
			err:          genericError,
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
			updateCalled:                true,
			err:                         nil,
		},
	},
	{
		"Lister success, state empty, ssl cert exists fails",
		in{
			lister:       listerSuccess,
			metrics:      fake.NewMetrics(),
			random:       randomSuccess,
			state:        empty(),
			sslExistsErr: genericError,
		},
		out{
			entryInState: true,
			updateCalled: false,
			err:          genericError,
		},
	},
	{
		"Lister success, entry in state, ssl cert exists fails",
		in{
			lister:       listerSuccess,
			metrics:      fake.NewMetrics(),
			random:       randomSuccess,
			state:        withEntry(),
			sslExistsErr: genericError,
		},
		out{
			entryInState: true,
			updateCalled: false,
			err:          genericError,
		},
	},
	{
		"Lister success, state empty, ssl cert create fails",
		in{
			lister:       listerSuccess,
			metrics:      fake.NewMetrics(),
			random:       randomSuccess,
			state:        empty(),
			sslCreateErr: genericError,
		},
		out{
			entryInState: true,
			updateCalled: false,
			err:          genericError,
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
			entryInState: true,
			updateCalled: false,
			err:          genericError,
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
			entryInState: true,
			updateCalled: false,
			err:          cnt_errors.ErrManagedCertificateNotFound,
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
			updateCalled:                true,
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
			updateCalled:                false,
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
			updateCalled:                false,
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
			entryInState: true,
			updateCalled: false,
			err:          genericError,
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
			entryInState: true,
			updateCalled: false,
			err:          genericError,
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
			entryInState:    false,
			wantSoftDeleted: true,
			updateCalled:    false,
			err:             cnt_errors.ErrSslCertificateOutOfSyncGotDeleted,
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
			entryInState:    false,
			wantSoftDeleted: true,
			updateCalled:    false,
			err:             cnt_errors.ErrSslCertificateOutOfSyncGotDeleted,
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
			entryInState:    true,
			wantSoftDeleted: true,
			updateCalled:    false,
			err:             genericError,
		},
	},
	{
		"Lister success, entry in state, certs mismatch, soft deleted in state",
		in{
			lister:  listerSuccess,
			metrics: fake.NewMetrics(),
			random:  randomSuccess,
			state:   withEntryAndSoftDeleted(),
			mcrt:    mockMcrt(domainBar),
		},
		out{
			entryInState:    false,
			wantSoftDeleted: true,
			updateCalled:    false,
			err:             cnt_errors.ErrSslCertificateOutOfSyncGotDeleted,
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
			entryInState: true,
			updateCalled: false,
			err:          cnt_errors.ErrManagedCertificateNotFound,
		},
	},
}

func TestManagedCertificate(t *testing.T) {
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			client := &fakenetworkingv1beta1.FakeNetworkingV1beta1{Fake: &cgo_testing.Fake{}}
			updateCalled := false
			client.AddReactor("update", "*", buildUpdateFunc(&updateCalled))

			config := config.NewFakeCertificateStatusConfig()
			ssl := newSsl(sslCertificateName, tc.in.mcrt, tc.in.sslCreateErr, tc.in.sslDeleteErr,
				tc.in.sslExistsErr, tc.in.sslGetErr)
			sut := New(client, config, tc.in.lister, tc.in.metrics, tc.in.random, ssl, tc.in.state)

			err := sut.ManagedCertificate(mcrtId)

			if _, err := tc.in.state.GetSslCertificateName(mcrtId); (err == nil) != tc.out.entryInState {
				t.Errorf("Entry in state %t, want %t, err: %v", err == nil, tc.out.entryInState, err)
			}

			if tc.out.wantSoftDeleted != tc.in.state.softDeleted {
				t.Errorf("Soft deleted: %t, want: %t", tc.in.state.softDeleted, tc.out.wantSoftDeleted)
			}

			reported, _ := tc.in.state.IsSslCertificateCreationReported(mcrtId)
			entryExists := tc.in.state.entryExists
			if entryExists != tc.out.entryInState || reported != tc.out.createLatencyMetricObserved {
				t.Errorf("Entry in state %t, want %t; CreateSslCertificateLatency metric observed %t, want %t",
					entryExists, tc.out.entryInState, reported, tc.out.createLatencyMetricObserved)
			}
			if tc.out.createLatencyMetricObserved && tc.in.metrics.SslCertificateCreationLatencyObserved != 1 {
				t.Errorf("CreateSslCertificateLatency metric observed %d times, want 1",
					tc.in.metrics.SslCertificateCreationLatencyObserved)
			}

			if tc.out.updateCalled != updateCalled {
				t.Errorf("Update called %t, want %t", updateCalled, tc.out.updateCalled)
			}

			if err != tc.out.err {
				t.Errorf("Have error: %v, want: %v", err, tc.out.err)
			}
		})
	}
}
