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

	api "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/gke.googleapis.com/v1alpha1"
	mcrtlister "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clientgen/listers/gke.googleapis.com/v1alpha1"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/config"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/fake"
)

const (
	domainBar          = "bar.com"
	domainFoo          = "foo.com"
	namespace          = "foo"
	name               = "bar"
	sslCertificateName = "baz"
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
			Namespace:         namespace,
			Name:              name,
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
	return newState(namespace, name, sslCertificateName)
}
func withEntryAndNoMetric() *fakeState {
	return newStateWithMetricOverride(namespace, name, sslCertificateName, false, false)
}
func withEntryAndMetricReported() *fakeState {
	return newStateWithMetricOverride(namespace, name, sslCertificateName, true, true)
}

type in struct {
	lister       mcrtlister.ManagedCertificateLister
	metrics      *fake.FakeMetrics
	random       fakeRandom
	state        *fakeState
	mcrt         *api.ManagedCertificate
	sslCreateErr []error
	sslDeleteErr []error
	sslExistsErr []error
	sslGetErr    []error
}

type out struct {
	entryInState                bool
	createLatencyMetricObserved bool
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
		"Lister fails with not found, entry in state, success to delete ssl cert",
		in{
			lister:  listerFailsNotFound,
			metrics: fake.NewMetrics(),
			random:  randomSuccess,
			state:   withEntry(),
		},
		out{
			entryInState: false,
			updateCalled: false,
			err:          nil,
		},
	},
	{
		"Lister fails with not found, entry in state, ssl cert already deleted",
		in{
			lister:       listerFailsNotFound,
			metrics:      fake.NewMetrics(),
			random:       randomSuccess,
			state:        withEntry(),
			sslDeleteErr: []error{googleNotFound},
		},
		out{
			entryInState: false,
			updateCalled: false,
			err:          nil,
		},
	},
	{
		"Lister fails with not found, entry in state, ssl cert fails to delete",
		in{
			lister:       listerFailsNotFound,
			metrics:      fake.NewMetrics(),
			random:       randomSuccess,
			state:        withEntry(),
			sslDeleteErr: []error{genericError},
		},
		out{
			entryInState: true,
			updateCalled: false,
			err:          genericError,
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
			sslExistsErr: []error{genericError},
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
			sslExistsErr: []error{genericError},
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
			sslCreateErr: []error{genericError},
		},
		out{
			entryInState: true,
			updateCalled: false,
			err:          genericError,
		},
	},
	{
		"Lister success, entry in state, ssl cert create fails",
		in{
			lister:       listerSuccess,
			metrics:      fake.NewMetrics(),
			random:       randomSuccess,
			state:        withEntry(),
			sslCreateErr: []error{genericError},
		},
		out{
			entryInState: true,
			updateCalled: false,
			err:          genericError,
		},
	},
	{
		"Lister success, entry in state, ssl cert create success, metric reported entry not found",
		in{
			lister:  listerSuccess,
			metrics: fake.NewMetrics(),
			random:  randomSuccess,
			state:   withEntryAndNoMetric(),
		},
		out{
			entryInState: true,
			updateCalled: false,
			err:          errManagedCertificateNotFound,
		},
	},
	{
		"Lister success, entry in state, ssl cert create success, metric already reported",
		in{
			lister:  listerSuccess,
			metrics: fake.NewMetricsSslCertificateCreationAlreadyReported(),
			random:  randomSuccess,
			state:   withEntryAndMetricReported(),
		},
		out{
			entryInState:                true,
			createLatencyMetricObserved: true,
			updateCalled:                true,
			err:                         nil,
		},
	},
	{
		"Lister success, state empty, ssl cert not exists - get fails",
		in{
			lister:    listerSuccess,
			metrics:   fake.NewMetrics(),
			random:    randomSuccess,
			state:     empty(),
			sslGetErr: []error{genericError},
		},
		out{
			entryInState:                true,
			createLatencyMetricObserved: true,
			updateCalled:                false,
			err:                         genericError,
		},
	},
	{
		"Lister success, entry in state, ssl cert not exists - get fails",
		in{
			lister:    listerSuccess,
			metrics:   fake.NewMetrics(),
			random:    randomSuccess,
			state:     withEntry(),
			sslGetErr: []error{genericError},
		},
		out{
			entryInState:                true,
			createLatencyMetricObserved: true,
			updateCalled:                false,
			err:                         genericError,
		},
	},
	{
		"Lister success, state empty, ssl cert exists - get fails",
		in{
			lister:    listerSuccess,
			metrics:   fake.NewMetrics(),
			random:    randomSuccess,
			state:     empty(),
			mcrt:      mockMcrt(domainFoo),
			sslGetErr: []error{genericError},
		},
		out{
			entryInState: true,
			updateCalled: false,
			err:          genericError,
		},
	},
	{
		"Lister success, entry in state, ssl cert exists - get fails",
		in{
			lister:    listerSuccess,
			metrics:   fake.NewMetrics(),
			random:    randomSuccess,
			state:     withEntry(),
			mcrt:      mockMcrt(domainFoo),
			sslGetErr: []error{genericError},
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
			entryInState: true,
			updateCalled: true,
			err:          nil,
		},
	},
	{
		"Lister success, entry in state, certs mismatch but ssl cert already deleted",
		in{
			lister:       listerSuccess,
			metrics:      fake.NewMetrics(),
			random:       randomSuccess,
			state:        withEntry(),
			mcrt:         mockMcrt(domainBar),
			sslDeleteErr: []error{googleNotFound},
		},
		out{
			entryInState: true,
			updateCalled: true,
			err:          nil,
		},
	},
	{
		"Lister success, entry in state, certs mismatch and deletion fails",
		in{
			lister:       listerSuccess,
			metrics:      fake.NewMetrics(),
			random:       randomSuccess,
			state:        withEntry(),
			mcrt:         mockMcrt(domainBar),
			sslDeleteErr: []error{genericError},
		},
		out{
			entryInState: true,
			updateCalled: false,
			err:          genericError,
		},
	},
	{
		"Lister success, entry in state, certs mismatch, ssl cert creation fails",
		in{
			lister:       listerSuccess,
			metrics:      fake.NewMetrics(),
			random:       randomSuccess,
			state:        withEntry(),
			mcrt:         mockMcrt(domainBar),
			sslCreateErr: []error{genericError},
		},
		out{
			entryInState: true,
			updateCalled: false,
			err:          genericError,
		},
	},
	{
		"Lister success, entry in state, certs mismatch but ssl cert already deleted, ssl cert creation fails",
		in{
			lister:       listerSuccess,
			metrics:      fake.NewMetrics(),
			random:       randomSuccess,
			state:        withEntry(),
			mcrt:         mockMcrt(domainBar),
			sslCreateErr: []error{genericError},
			sslDeleteErr: []error{googleNotFound},
		},
		out{
			entryInState: true,
			updateCalled: false,
			err:          genericError,
		},
	},
	{
		"Lister success, entry in state, certs mismatch, ssl cert get fails",
		in{
			lister:    listerSuccess,
			metrics:   fake.NewMetrics(),
			random:    randomSuccess,
			state:     withEntry(),
			mcrt:      mockMcrt(domainBar),
			sslGetErr: []error{nil, genericError},
		},
		out{
			entryInState: true,
			updateCalled: false,
			err:          genericError,
		},
	},
	{
		"Lister success, entry in state, certs mismatch but ssl cert already deleted, ssl cert get fails",
		in{
			lister:       listerSuccess,
			metrics:      fake.NewMetrics(),
			random:       randomSuccess,
			state:        withEntry(),
			mcrt:         mockMcrt(domainBar),
			sslDeleteErr: []error{googleNotFound},
			sslGetErr:    []error{nil, genericError},
		},
		out{
			entryInState: true,
			updateCalled: false,
			err:          genericError,
		},
	},
}

func TestManagedCertificate(t *testing.T) {
	for _, testCase := range testCases {
		t.Run(testCase.desc, func(t *testing.T) {
			clientset := newClientset()
			updateCalled := false
			clientset.AddReactor("update", "*", buildUpdateFunc(&updateCalled))

			config := config.NewFakeCertificateStatusConfig()
			ssl := newSsl(sslCertificateName, testCase.in.mcrt, testCase.in.sslCreateErr,
				testCase.in.sslDeleteErr, testCase.in.sslExistsErr, testCase.in.sslGetErr)
			sut := New(clientset, config, testCase.in.lister, testCase.in.metrics, testCase.in.random, ssl,
				testCase.in.state)

			err := sut.ManagedCertificate(namespace, name)

			if _, e := testCase.in.state.GetSslCertificateName(namespace, name); e != testCase.out.entryInState {
				t.Errorf("Entry in state %t, want %t", e, testCase.out.entryInState)
			}

			reported, _ := testCase.in.state.IsSslCertificateCreationReported(namespace, name)
			entryExists := testCase.in.state.entryExists
			if entryExists != testCase.out.entryInState || reported != testCase.out.createLatencyMetricObserved {
				t.Errorf("Entry in state %t, want %t; CreateSslCertificateLatency metric observed %t, want %t",
					entryExists, testCase.out.entryInState, reported, testCase.out.createLatencyMetricObserved)
			}
			if testCase.out.createLatencyMetricObserved && testCase.in.metrics.SslCertificateCreationLatencyObserved != 1 {
				t.Errorf("CreateSslCertificateLatency metric observed %d times, want 1",
					testCase.in.metrics.SslCertificateCreationLatencyObserved)
			}

			if testCase.out.updateCalled != updateCalled {
				t.Errorf("Update called %t, want %t", updateCalled, testCase.out.updateCalled)
			}

			if err != testCase.out.err {
				t.Errorf("Have error: %v, want: %v", err, testCase.out.err)
			}
		})
	}
}
