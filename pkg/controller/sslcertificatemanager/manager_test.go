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

	apisv1beta2 "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/networking.gke.io/v1beta2"
	compute "google.golang.org/api/compute/v0.beta"
	"google.golang.org/api/googleapi"

	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clients/event"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clients/ssl"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/fake"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/types"
)

var (
	domain           = "foo.com"
	cert             = &compute.SslCertificate{}
	certId           = types.NewCertId("default", "bar")
	errGeneric       = errors.New("generic error")
	errQuotaExceeded = ssl.NewFakeQuotaExceededError()
	errNotFound      = &googleapi.Error{Code: 404}
)

type fakeSsl struct {
	err            error
	exists         bool
	sslCertificate *compute.SslCertificate
}

var _ ssl.Ssl = (*fakeSsl)(nil)

func (f fakeSsl) Create(ctx context.Context, name string, domains []string) error {
	return f.err
}

func (f fakeSsl) Delete(ctx context.Context, name string) error {
	return f.err
}

func (f fakeSsl) Exists(name string) (bool, error) {
	return f.exists, f.err
}

func (f fakeSsl) Get(name string) (*compute.SslCertificate, error) {
	return f.sslCertificate, f.err
}

func (f fakeSsl) List() ([]*compute.SslCertificate, error) {
	return nil, errors.New("not implemented")
}

func withErr(err error) fakeSsl {
	return fakeSsl{
		err:            err,
		exists:         false,
		sslCertificate: nil,
	}
}

func withExists(err error, exists bool) fakeSsl {
	return fakeSsl{
		err:            err,
		exists:         exists,
		sslCertificate: nil,
	}
}

func withCert(err error, sslCertificate *compute.SslCertificate) fakeSsl {
	return fakeSsl{
		err:            err,
		exists:         false,
		sslCertificate: sslCertificate,
	}
}

type fakeEvent struct {
	backendErrorCnt int
	createCnt       int
	deleteCnt       int
	tooManyCnt      int
}

var _ event.Event = (*fakeEvent)(nil)

func (f *fakeEvent) BackendError(mcrt apisv1beta2.ManagedCertificate, err error) {
	f.backendErrorCnt++
}

func (f *fakeEvent) Create(mcrt apisv1beta2.ManagedCertificate, sslCertificateName string) {
	f.createCnt++
}

func (f *fakeEvent) Delete(mcrt apisv1beta2.ManagedCertificate, sslCertificateName string) {
	f.deleteCnt++
}

func (f *fakeEvent) TooManyCertificates(mcrt apisv1beta2.ManagedCertificate, err error) {
	f.tooManyCnt++
}

func TestCreate(t *testing.T) {
	testCases := []struct {
		ssl                   ssl.Ssl
		excludedFromSLOErr    error
		wantErr               error
		wantTooManyCertsEvent bool
		wantExcludedFromSLO   bool
		wantBackendErrorEvent bool
		wantCreateEvent       bool
	}{
		{
			ssl:             withErr(nil),
			wantCreateEvent: true,
		},
		{
			ssl:                   withErr(errQuotaExceeded),
			wantErr:               errQuotaExceeded,
			wantTooManyCertsEvent: true,
			wantExcludedFromSLO:   true,
		},
		{
			ssl:                   withErr(errQuotaExceeded),
			excludedFromSLOErr:    errGeneric,
			wantErr:               errGeneric,
			wantTooManyCertsEvent: true,
		},
		{
			ssl:                   withErr(errGeneric),
			wantErr:               errGeneric,
			wantBackendErrorEvent: true,
		},
	}

	for _, tc := range testCases {
		ctx := context.Background()

		event := &fakeEvent{0, 0, 0, 0}
		metrics := fake.NewMetrics()
		state := fake.NewStateWithEntries(map[types.CertId]fake.StateEntry{
			certId: fake.StateEntry{SslCertificateName: "", ExcludedFromSLOErr: tc.excludedFromSLOErr},
		})
		sut := New(event, metrics, tc.ssl, state)

		err := sut.Create(ctx, "", *fake.NewManagedCertificate(certId, domain))

		if err != tc.wantErr {
			t.Fatalf("err %#v, want %#v", err, tc.wantErr)
		}

		oneTooManyCertsEvent := event.tooManyCnt == 1
		if tc.wantTooManyCertsEvent != oneTooManyCertsEvent {
			t.Fatalf("TooManyCertificates events generated: %d, want event: %t", event.tooManyCnt, tc.wantTooManyCertsEvent)
		}

		oneSslCertificateQuotaErrorObserved := metrics.SslCertificateQuotaErrorObserved == 1
		if tc.wantTooManyCertsEvent != oneSslCertificateQuotaErrorObserved {
			t.Fatalf("Metric SslCertificateQuotaError observed %d times", metrics.SslCertificateQuotaErrorObserved)
		}

		excluded, err := state.IsExcludedFromSLO(certId)
		if tc.excludedFromSLOErr != err || excluded != tc.wantExcludedFromSLO {
			t.Fatalf("Excluded from SLO is %t, err %v; want %t, err %v", excluded, err,
				tc.wantExcludedFromSLO, tc.excludedFromSLOErr)
		}

		oneBackendErrorEvent := event.backendErrorCnt == 1
		if tc.wantBackendErrorEvent != oneBackendErrorEvent {
			t.Fatalf("BackendError events generated: %d, want event: %t", event.backendErrorCnt, tc.wantBackendErrorEvent)
		}

		oneSslCertificateBackendErrorObserved := metrics.SslCertificateBackendErrorObserved == 1
		if tc.wantBackendErrorEvent != oneSslCertificateBackendErrorObserved {
			t.Fatalf("Metric SslCertificateBackendError observed %d times", metrics.SslCertificateBackendErrorObserved)
		}

		oneCreateEvent := event.createCnt == 1
		if tc.wantCreateEvent != oneCreateEvent {
			t.Fatalf("Create events generated: %d, want event: %t", event.createCnt, tc.wantCreateEvent)
		}
	}
}

func TestDelete(t *testing.T) {
	testCases := []struct {
		ssl             ssl.Ssl
		mcrt            *apisv1beta2.ManagedCertificate
		wantErr         error
		wantErrorEvent  bool
		wantDeleteEvent bool
	}{
		{
			ssl: withErr(nil),
		},
		{
			ssl:             withErr(nil),
			mcrt:            fake.NewManagedCertificate(certId, domain),
			wantDeleteEvent: true,
		},
		{
			ssl:     withErr(errGeneric),
			wantErr: errGeneric,
		},
		{
			ssl:            withErr(errGeneric),
			mcrt:           fake.NewManagedCertificate(certId, domain),
			wantErr:        errGeneric,
			wantErrorEvent: true,
		},
		{
			ssl: withErr(errNotFound),
		},
		{
			ssl:  withErr(errNotFound),
			mcrt: fake.NewManagedCertificate(certId, domain),
		},
	}

	for _, tc := range testCases {
		ctx := context.Background()

		event := &fakeEvent{0, 0, 0, 0}
		metrics := fake.NewMetrics()
		sut := New(event, metrics, tc.ssl, fake.NewState())

		err := sut.Delete(ctx, "", tc.mcrt)

		if err != tc.wantErr {
			t.Fatalf("err %#v, want %#v", err, tc.wantErr)
		}

		oneBackendErrorEvent := event.backendErrorCnt == 1
		if tc.wantErrorEvent != oneBackendErrorEvent {
			t.Fatalf("BackendError events generated: %d, want event: %t", event.backendErrorCnt, tc.wantErrorEvent)
		}

		oneSslCertificateBackendErrorObserved := metrics.SslCertificateBackendErrorObserved == 1
		if tc.wantErrorEvent != oneSslCertificateBackendErrorObserved {
			t.Fatalf("Metric SslCertificateBackendError observed %d times", metrics.SslCertificateBackendErrorObserved)
		}

		oneDeleteEvent := event.deleteCnt == 1
		if tc.wantDeleteEvent != oneDeleteEvent {
			t.Fatalf("Delete events generated: %d, want event: %t", event.deleteCnt, tc.wantDeleteEvent)
		}
	}
}

func TestExists(t *testing.T) {
	testCases := []struct {
		ssl            ssl.Ssl
		mcrt           *apisv1beta2.ManagedCertificate
		wantExists     bool
		wantErr        error
		wantErrorEvent bool
	}{
		{
			ssl: withExists(nil, false),
		},
		{
			ssl:  withExists(nil, false),
			mcrt: fake.NewManagedCertificate(certId, domain),
		},
		{
			ssl:        withExists(nil, true),
			wantExists: true,
		},
		{
			ssl:        withExists(nil, true),
			mcrt:       fake.NewManagedCertificate(certId, domain),
			wantExists: true,
		},
		{
			ssl:     withExists(errGeneric, false),
			wantErr: errGeneric,
		},
		{
			ssl:            withExists(errGeneric, false),
			mcrt:           fake.NewManagedCertificate(certId, domain),
			wantErr:        errGeneric,
			wantErrorEvent: true,
		},
		{
			ssl:     withExists(errGeneric, true),
			wantErr: errGeneric,
		},
		{
			ssl:            withExists(errGeneric, true),
			mcrt:           fake.NewManagedCertificate(certId, domain),
			wantErr:        errGeneric,
			wantErrorEvent: true,
		},
	}

	for _, tc := range testCases {
		event := &fakeEvent{0, 0, 0, 0}
		metrics := fake.NewMetrics()
		sut := New(event, metrics, tc.ssl, fake.NewState())

		exists, err := sut.Exists("", tc.mcrt)

		if err != tc.wantErr {
			t.Fatalf("err %#v, want %#v", err, tc.wantErr)
		} else if exists != tc.wantExists {
			t.Fatalf("exists: %t, want %t", exists, tc.wantExists)
		}

		oneErrorEvent := event.backendErrorCnt == 1
		if tc.wantErrorEvent != oneErrorEvent {
			t.Fatalf("BackendError events generated: %d, want event: %t", event.backendErrorCnt, tc.wantErrorEvent)
		}

		oneSslCertificateBackendErrorObserved := metrics.SslCertificateBackendErrorObserved == 1
		if tc.wantErrorEvent != oneSslCertificateBackendErrorObserved {
			t.Fatalf("Metric SslCertificateBackendError observed %d times", metrics.SslCertificateBackendErrorObserved)
		}
	}
}

func TestGet(t *testing.T) {
	testCases := []struct {
		ssl            ssl.Ssl
		mcrt           *apisv1beta2.ManagedCertificate
		wantCert       *compute.SslCertificate
		wantErr        error
		wantErrorEvent bool
	}{
		{
			ssl: withCert(nil, nil),
		},
		{
			ssl:  withCert(nil, nil),
			mcrt: fake.NewManagedCertificate(certId, domain),
		},
		{
			ssl:      withCert(nil, cert),
			wantCert: cert,
		},
		{
			ssl:      withCert(nil, cert),
			mcrt:     fake.NewManagedCertificate(certId, domain),
			wantCert: cert,
		},
		{
			ssl:     withCert(errGeneric, nil),
			wantErr: errGeneric,
		},
		{
			ssl:            withCert(errGeneric, nil),
			mcrt:           fake.NewManagedCertificate(certId, domain),
			wantErr:        errGeneric,
			wantErrorEvent: true,
		},
		{
			ssl:     withCert(errGeneric, cert),
			wantErr: errGeneric,
		},
		{
			ssl:            withCert(errGeneric, cert),
			mcrt:           fake.NewManagedCertificate(certId, domain),
			wantErr:        errGeneric,
			wantErrorEvent: true,
		},
	}

	for _, tc := range testCases {
		event := &fakeEvent{0, 0, 0, 0}
		metrics := fake.NewMetrics()
		sut := New(event, metrics, tc.ssl, fake.NewState())

		sslCert, err := sut.Get("", tc.mcrt)

		if err != tc.wantErr {
			t.Fatalf("err %#v, want %#v", err, tc.wantErr)
		} else if sslCert != tc.wantCert {
			t.Fatalf("cert: %#v, want %#v", sslCert, tc.wantCert)
		}

		oneErrorEvent := event.backendErrorCnt == 1
		if tc.wantErrorEvent != oneErrorEvent {
			t.Fatalf("BackendError events generated: %d, want event: %t", event.backendErrorCnt, tc.wantErrorEvent)
		}

		oneSslCertificateBackendErrorObserved := metrics.SslCertificateBackendErrorObserved == 1
		if tc.wantErrorEvent != oneSslCertificateBackendErrorObserved {
			t.Fatalf("Metric SslCertificateBackendError observed %d times", metrics.SslCertificateBackendErrorObserved)
		}
	}
}
