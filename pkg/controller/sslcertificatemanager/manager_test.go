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

package sslcertificatemanager

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	computev1 "google.golang.org/api/compute/v1"

	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/networking.gke.io/v1"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clients/event"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clients/ssl"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/metrics"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/state"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/testhelper/managedcertificate"
	utilserrors "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/errors"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/types"
)

var (
	errFake  = errors.New("fake error")
	errQuota = ssl.NewFakeQuotaExceededError()
)

type sslError struct {
	ssl.Fake

	err    error
	exists bool
}

func (s *sslError) Create(ctx context.Context, name string, domains []string) error {
	return s.err
}

func (s *sslError) Delete(ctx context.Context, name string) error {
	return s.err
}

func (s *sslError) Exists(name string) (bool, error) {
	return s.exists, s.err
}

func (s *sslError) Get(name string) (*computev1.SslCertificate, error) {
	return nil, s.err
}

func TestCreate(t *testing.T) {
	t.Parallel()

	for description, testCase := range map[string]struct {
		ssl                ssl.Interface
		name               string
		managedCertificate *v1.ManagedCertificate
		state              state.Interface

		wantErr     error
		wantSsl     ssl.Interface
		wantState   state.Interface
		wantEvent   event.Fake
		wantMetrics metrics.Fake
	}{
		"happy path": {
			ssl:                ssl.NewFake().Build(),
			name:               "foo",
			managedCertificate: managedcertificate.New(types.NewId("default", "foo"), "example.com").Build(),
			state:              state.NewFake(),

			wantSsl:   ssl.NewFake().AddEntry("foo", []string{"example.com"}).Build(),
			wantState: state.NewFake(),
			wantEvent: event.Fake{CreateCnt: 1},
		},
		"quota exceeded, empty state": {
			ssl:                &sslError{err: errQuota},
			name:               "foo",
			managedCertificate: managedcertificate.New(types.NewId("default", "foo"), "example.com").Build(),
			state:              state.NewFake(),

			wantErr:     utilserrors.NotFound,
			wantSsl:     ssl.NewFake().Build(),
			wantState:   state.NewFake(),
			wantEvent:   event.Fake{TooManyCertificatesCnt: 1},
			wantMetrics: metrics.Fake{QuotaErrorCnt: 1},
		},
		"quota exceeded, entry in state": {
			ssl:                &sslError{err: errQuota},
			name:               "foo",
			managedCertificate: managedcertificate.New(types.NewId("default", "foo"), "example.com").Build(),
			state: state.NewFakeWithEntries(map[types.Id]state.Entry{
				types.NewId("default", "foo"): state.Entry{
					SslCertificateName: "foo",
				},
			}),

			wantErr: errQuota,
			wantSsl: ssl.NewFake().Build(),
			wantState: state.NewFakeWithEntries(map[types.Id]state.Entry{
				types.NewId("default", "foo"): state.Entry{
					SslCertificateName: "foo",
					ExcludedFromSLO:    true,
				},
			}),
			wantEvent:   event.Fake{TooManyCertificatesCnt: 1},
			wantMetrics: metrics.Fake{QuotaErrorCnt: 1},
		},
		"other error": {
			ssl:                &sslError{err: errFake},
			name:               "foo",
			managedCertificate: managedcertificate.New(types.NewId("default", "foo"), "example.com").Build(),
			state: state.NewFakeWithEntries(map[types.Id]state.Entry{
				types.NewId("default", "foo"): state.Entry{
					SslCertificateName: "foo",
				},
			}),

			wantErr: errFake,
			wantSsl: ssl.NewFake().Build(),
			wantState: state.NewFakeWithEntries(map[types.Id]state.Entry{
				types.NewId("default", "foo"): state.Entry{
					SslCertificateName: "foo",
				},
			}),
			wantEvent:   event.Fake{BackendErrorCnt: 1},
			wantMetrics: metrics.Fake{BackendErrorCnt: 1},
		},
	} {
		testCase := testCase
		t.Run(description, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()

			event := event.Fake{}
			metrics := metrics.NewFake()
			manager := New(&event, metrics, testCase.ssl, testCase.state)

			err := manager.Create(ctx, testCase.name, *testCase.managedCertificate)

			if err != testCase.wantErr {
				t.Fatalf("Create(): %v, want: %v", err, testCase.wantErr)
			}

			wantSsl, _ := testCase.wantSsl.List()
			gotSsl, _ := testCase.ssl.List()
			if diff := cmp.Diff(wantSsl, gotSsl); diff != "" {
				t.Fatalf("Diff ssl (-want, +got): %s", diff)
			}

			if diff := cmp.Diff(testCase.wantState.List(), testCase.state.List()); diff != "" {
				t.Fatalf("Diff state (-want, +got): %s", diff)
			}

			if diff := cmp.Diff(testCase.wantEvent, event); diff != "" {
				t.Fatalf("Diff event (-want, +got): %s", diff)
			}

			if diff := cmp.Diff(testCase.wantMetrics, *metrics); diff != "" {
				t.Fatalf("Diff metrics (-want, +got): %s", diff)
			}
		})
	}
}

func TestDelete(t *testing.T) {
	t.Parallel()

	for description, testCase := range map[string]struct {
		ssl                ssl.Interface
		name               string
		managedCertificate *v1.ManagedCertificate

		wantErr     error
		wantSsl     ssl.Interface
		wantEvent   event.Fake
		wantMetrics metrics.Fake
	}{
		"happy path with ManagedCertificate": {
			ssl:                ssl.NewFake().AddEntry("foo", []string{"example.com"}).Build(),
			name:               "foo",
			managedCertificate: managedcertificate.New(types.NewId("default", "foo"), "example.com").Build(),

			wantSsl:   ssl.NewFake().Build(),
			wantEvent: event.Fake{DeleteCnt: 1},
		},
		"happy path without ManagedCertificate": {
			ssl:  ssl.NewFake().AddEntry("foo", []string{"example.com"}).Build(),
			name: "foo",

			wantSsl: ssl.NewFake().Build(),
		},
		"not found with ManagedCertificate": {
			ssl:                ssl.NewFake().Build(),
			name:               "foo",
			managedCertificate: managedcertificate.New(types.NewId("default", "foo"), "example.com").Build(),

			wantSsl: ssl.NewFake().Build(),
		},
		"not found without ManagedCertificate": {
			ssl:  ssl.NewFake().Build(),
			name: "foo",

			wantSsl: ssl.NewFake().Build(),
		},
		"other error with ManagedCertificate": {
			ssl:                &sslError{err: errFake},
			name:               "foo",
			managedCertificate: managedcertificate.New(types.NewId("default", "foo"), "example.com").Build(),

			wantErr:     errFake,
			wantSsl:     ssl.NewFake().Build(),
			wantEvent:   event.Fake{BackendErrorCnt: 1},
			wantMetrics: metrics.Fake{BackendErrorCnt: 1},
		},
		"other error without ManagedCertificate": {
			ssl:  &sslError{err: errFake},
			name: "foo",

			wantErr:     errFake,
			wantSsl:     ssl.NewFake().Build(),
			wantMetrics: metrics.Fake{BackendErrorCnt: 1},
		},
	} {
		testCase := testCase
		t.Run(description, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()

			event := event.Fake{}
			metrics := metrics.NewFake()
			manager := New(&event, metrics, testCase.ssl, state.NewFake())

			err := manager.Delete(ctx, testCase.name, testCase.managedCertificate)

			if err != testCase.wantErr {
				t.Fatalf("Delete(): %v, want: %v", err, testCase.wantErr)
			}

			wantSsl, _ := testCase.wantSsl.List()
			gotSsl, _ := testCase.ssl.List()
			if diff := cmp.Diff(wantSsl, gotSsl); diff != "" {
				t.Fatalf("Diff ssl (-want, +got): %s", diff)
			}

			if diff := cmp.Diff(testCase.wantEvent, event); diff != "" {
				t.Fatalf("Diff event (-want, +got): %s", diff)
			}

			if diff := cmp.Diff(&testCase.wantMetrics, metrics); diff != "" {
				t.Fatalf("Diff metrics (-want, +got): %s", diff)
			}
		})
	}
}

func TestExists(t *testing.T) {
	t.Parallel()

	for description, testCase := range map[string]struct {
		ssl                ssl.Interface
		name               string
		managedCertificate *v1.ManagedCertificate

		wantExists  bool
		wantErr     error
		wantEvent   event.Fake
		wantMetrics metrics.Fake
	}{
		"happy path with ManagedCertificate": {
			ssl:                ssl.NewFake().AddEntry("foo", []string{"example.com"}).Build(),
			name:               "foo",
			managedCertificate: managedcertificate.New(types.NewId("default", "foo"), "example.com").Build(),

			wantExists: true,
		},
		"happy path without ManagedCertificate": {
			ssl:  ssl.NewFake().AddEntry("foo", []string{"example.com"}).Build(),
			name: "foo",

			wantExists: true,
		},
		"not found with ManagedCertificate": {
			ssl:                ssl.NewFake().Build(),
			name:               "foo",
			managedCertificate: managedcertificate.New(types.NewId("default", "foo"), "example.com").Build(),

			wantExists: false,
		},
		"not found without ManagedCertificate": {
			ssl:  ssl.NewFake().Build(),
			name: "foo",

			wantExists: false,
		},
		"other error with ManagedCertificate": {
			ssl:                &sslError{err: errFake, exists: false},
			name:               "foo",
			managedCertificate: managedcertificate.New(types.NewId("default", "foo"), "example.com").Build(),

			wantExists:  false,
			wantErr:     errFake,
			wantEvent:   event.Fake{BackendErrorCnt: 1},
			wantMetrics: metrics.Fake{BackendErrorCnt: 1},
		},
		"other error without ManagedCertificate": {
			ssl:  &sslError{err: errFake, exists: false},
			name: "foo",

			wantExists:  false,
			wantErr:     errFake,
			wantMetrics: metrics.Fake{BackendErrorCnt: 1},
		},
	} {
		testCase := testCase
		t.Run(description, func(t *testing.T) {
			t.Parallel()

			event := event.Fake{}
			metrics := metrics.NewFake()
			manager := New(&event, metrics, testCase.ssl, state.NewFake())

			exists, err := manager.Exists(testCase.name, testCase.managedCertificate)

			if exists != testCase.wantExists || err != testCase.wantErr {
				t.Fatalf("Exists(): %t, %v, want %t, %v", exists, err, testCase.wantExists, testCase.wantErr)
			}

			if diff := cmp.Diff(testCase.wantEvent, event); diff != "" {
				t.Fatalf("Diff event (-want, +got): %s", diff)
			}

			if diff := cmp.Diff(&testCase.wantMetrics, metrics); diff != "" {
				t.Fatalf("Diff metrics (-want, +got): %s", diff)
			}
		})
	}
}

func TestGet(t *testing.T) {
	t.Parallel()

	for description, testCase := range map[string]struct {
		ssl                ssl.Interface
		name               string
		managedCertificate *v1.ManagedCertificate

		wantCert    *computev1.SslCertificate
		wantErr     error
		wantEvent   event.Fake
		wantMetrics metrics.Fake
	}{
		"happy path with ManagedCertificate": {
			ssl:                ssl.NewFake().AddEntry("foo", []string{"example.com"}).Build(),
			name:               "foo",
			managedCertificate: managedcertificate.New(types.NewId("default", "foo"), "example.com").Build(),

			wantCert: ssl.NewFakeSslCertificate("foo", "", map[string]string{"example.com": ""}),
		},
		"happy path without ManagedCertificate": {
			ssl:  ssl.NewFake().AddEntry("foo", []string{"example.com"}).Build(),
			name: "foo",

			wantCert: ssl.NewFakeSslCertificate("foo", "", map[string]string{"example.com": ""}),
		},
		"not found with ManagedCertificate": {
			ssl:                ssl.NewFake().Build(),
			name:               "foo",
			managedCertificate: managedcertificate.New(types.NewId("default", "foo"), "example.com").Build(),

			wantCert:    nil,
			wantErr:     utilserrors.NotFound,
			wantEvent:   event.Fake{BackendErrorCnt: 1},
			wantMetrics: metrics.Fake{BackendErrorCnt: 1},
		},
		"not found without ManagedCertificate": {
			ssl:  ssl.NewFake().Build(),
			name: "foo",

			wantCert:    nil,
			wantErr:     utilserrors.NotFound,
			wantMetrics: metrics.Fake{BackendErrorCnt: 1},
		},
		"other error with ManagedCertificate": {
			ssl:                &sslError{err: errFake},
			name:               "foo",
			managedCertificate: managedcertificate.New(types.NewId("default", "foo"), "example.com").Build(),

			wantCert:    nil,
			wantErr:     errFake,
			wantEvent:   event.Fake{BackendErrorCnt: 1},
			wantMetrics: metrics.Fake{BackendErrorCnt: 1},
		},
		"other error without ManagedCertificate": {
			ssl:  &sslError{err: errFake},
			name: "foo",

			wantCert:    nil,
			wantErr:     errFake,
			wantMetrics: metrics.Fake{BackendErrorCnt: 1},
		},
	} {
		testCase := testCase
		t.Run(description, func(t *testing.T) {
			t.Parallel()

			event := event.Fake{}
			metrics := metrics.NewFake()
			manager := New(&event, metrics, testCase.ssl, state.NewFake())

			cert, err := manager.Get(testCase.name, testCase.managedCertificate)

			if diff := cmp.Diff(testCase.wantCert, cert); diff != "" {
				t.Fatalf("Diff SslCertificates (-want, +got): %s", diff)
			}

			if err != testCase.wantErr {
				t.Fatalf("Get(): %v, want %v", err, testCase.wantErr)
			}

			if diff := cmp.Diff(testCase.wantEvent, event); diff != "" {
				t.Fatalf("Diff event (-want, +got): %s", diff)
			}

			if diff := cmp.Diff(&testCase.wantMetrics, metrics); diff != "" {
				t.Fatalf("Diff metrics (-want, +got): %s", diff)
			}
		})
	}
}
