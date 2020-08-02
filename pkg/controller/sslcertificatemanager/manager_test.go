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

	apisv1 "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/networking.gke.io/v1"
	compute "google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"

	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clients/event"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clients/ssl"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/metrics"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/state"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/testhelper/managedcertificate"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/types"
)

var (
	domain           = "foo.com"
	cert             = &compute.SslCertificate{}
	certId           = types.NewId("default", "bar")
	errGeneric       = errors.New("generic error")
	errQuotaExceeded = ssl.NewFakeQuotaExceededError()
	errNotFound      = &googleapi.Error{Code: 404}
)

type fakeSsl struct {
	err            error
	exists         bool
	sslCertificate *compute.SslCertificate
}

var _ ssl.Interface = (*fakeSsl)(nil)

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

func TestCreate(t *testing.T) {
	testCases := []struct {
		ssl                   ssl.Interface
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
			ssl:                   withErr(errGeneric),
			wantErr:               errGeneric,
			wantBackendErrorEvent: true,
		},
	}

	for _, tc := range testCases {
		ctx := context.Background()

		event := &event.Fake{}
		metrics := metrics.NewFake()
		state := state.NewFakeWithEntries(map[types.Id]state.Entry{
			certId: state.Entry{SslCertificateName: ""},
		})
		sut := New(event, metrics, tc.ssl, state)

		err := sut.Create(ctx, "", *managedcertificate.New(certId, domain).Build())

		if err != tc.wantErr {
			t.Fatalf("err %#v, want %#v", err, tc.wantErr)
		}

		oneTooManyCertsEvent := event.TooManyCnt == 1
		if tc.wantTooManyCertsEvent != oneTooManyCertsEvent {
			t.Fatalf("TooManyCertificates events generated: %d, want event: %t",
				event.TooManyCnt, tc.wantTooManyCertsEvent)
		}

		oneSslCertificateQuotaErrorObserved := metrics.SslCertificateQuotaErrorObserved == 1
		if tc.wantTooManyCertsEvent != oneSslCertificateQuotaErrorObserved {
			t.Fatalf("Metric SslCertificateQuotaError observed %d times",
				metrics.SslCertificateQuotaErrorObserved)
		}

		entry, err := state.Get(certId)
		if err != nil {
			t.Fatalf("state.Get(%s): %v, want nil", certId.String(), err)
		}
		if entry.ExcludedFromSLO != tc.wantExcludedFromSLO {
			t.Fatalf("Excluded from SLO is %t, want %t",
				entry.ExcludedFromSLO, tc.wantExcludedFromSLO)
		}

		oneBackendErrorEvent := event.BackendErrorCnt == 1
		if tc.wantBackendErrorEvent != oneBackendErrorEvent {
			t.Fatalf("BackendError events generated: %d, want event: %t",
				event.BackendErrorCnt, tc.wantBackendErrorEvent)
		}

		oneSslCertificateBackendErrorObserved := metrics.SslCertificateBackendErrorObserved == 1
		if tc.wantBackendErrorEvent != oneSslCertificateBackendErrorObserved {
			t.Fatalf("Metric SslCertificateBackendError observed %d times",
				metrics.SslCertificateBackendErrorObserved)
		}

		oneCreateEvent := event.CreateCnt == 1
		if tc.wantCreateEvent != oneCreateEvent {
			t.Fatalf("Create events generated: %d, want event: %t",
				event.CreateCnt, tc.wantCreateEvent)
		}
	}
}

func TestDelete(t *testing.T) {
	testCases := []struct {
		ssl             ssl.Interface
		mcrt            *apisv1.ManagedCertificate
		wantErr         error
		wantErrorEvent  bool
		wantDeleteEvent bool
	}{
		{
			ssl: withErr(nil),
		},
		{
			ssl:             withErr(nil),
			mcrt:            managedcertificate.New(certId, domain).Build(),
			wantDeleteEvent: true,
		},
		{
			ssl:     withErr(errGeneric),
			wantErr: errGeneric,
		},
		{
			ssl:            withErr(errGeneric),
			mcrt:           managedcertificate.New(certId, domain).Build(),
			wantErr:        errGeneric,
			wantErrorEvent: true,
		},
		{
			ssl: withErr(errNotFound),
		},
		{
			ssl:  withErr(errNotFound),
			mcrt: managedcertificate.New(certId, domain).Build(),
		},
	}

	for _, tc := range testCases {
		ctx := context.Background()

		event := &event.Fake{}
		metrics := metrics.NewFake()
		sut := New(event, metrics, tc.ssl, state.NewFake())

		err := sut.Delete(ctx, "", tc.mcrt)

		if err != tc.wantErr {
			t.Fatalf("err %#v, want %#v", err, tc.wantErr)
		}

		oneBackendErrorEvent := event.BackendErrorCnt == 1
		if tc.wantErrorEvent != oneBackendErrorEvent {
			t.Fatalf("BackendError events generated: %d, want event: %t",
				event.BackendErrorCnt, tc.wantErrorEvent)
		}

		oneSslCertificateBackendErrorObserved := metrics.SslCertificateBackendErrorObserved == 1
		if tc.wantErrorEvent != oneSslCertificateBackendErrorObserved {
			t.Fatalf("Metric SslCertificateBackendError observed %d times",
				metrics.SslCertificateBackendErrorObserved)
		}

		oneDeleteEvent := event.DeleteCnt == 1
		if tc.wantDeleteEvent != oneDeleteEvent {
			t.Fatalf("Delete events generated: %d, want event: %t",
				event.DeleteCnt, tc.wantDeleteEvent)
		}
	}
}

func TestExists(t *testing.T) {
	testCases := []struct {
		ssl            ssl.Interface
		mcrt           *apisv1.ManagedCertificate
		wantExists     bool
		wantErr        error
		wantErrorEvent bool
	}{
		{
			ssl: withExists(nil, false),
		},
		{
			ssl:  withExists(nil, false),
			mcrt: managedcertificate.New(certId, domain).Build(),
		},
		{
			ssl:        withExists(nil, true),
			wantExists: true,
		},
		{
			ssl:        withExists(nil, true),
			mcrt:       managedcertificate.New(certId, domain).Build(),
			wantExists: true,
		},
		{
			ssl:     withExists(errGeneric, false),
			wantErr: errGeneric,
		},
		{
			ssl:            withExists(errGeneric, false),
			mcrt:           managedcertificate.New(certId, domain).Build(),
			wantErr:        errGeneric,
			wantErrorEvent: true,
		},
		{
			ssl:     withExists(errGeneric, true),
			wantErr: errGeneric,
		},
		{
			ssl:            withExists(errGeneric, true),
			mcrt:           managedcertificate.New(certId, domain).Build(),
			wantErr:        errGeneric,
			wantErrorEvent: true,
		},
	}

	for _, tc := range testCases {
		event := &event.Fake{}
		metrics := metrics.NewFake()
		sut := New(event, metrics, tc.ssl, state.NewFake())

		exists, err := sut.Exists("", tc.mcrt)

		if err != tc.wantErr {
			t.Fatalf("err %#v, want %#v", err, tc.wantErr)
		} else if exists != tc.wantExists {
			t.Fatalf("exists: %t, want %t", exists, tc.wantExists)
		}

		oneErrorEvent := event.BackendErrorCnt == 1
		if tc.wantErrorEvent != oneErrorEvent {
			t.Fatalf("BackendError events generated: %d, want event: %t",
				event.BackendErrorCnt, tc.wantErrorEvent)
		}

		oneSslCertificateBackendErrorObserved := metrics.SslCertificateBackendErrorObserved == 1
		if tc.wantErrorEvent != oneSslCertificateBackendErrorObserved {
			t.Fatalf("Metric SslCertificateBackendError observed %d times",
				metrics.SslCertificateBackendErrorObserved)
		}
	}
}

func TestGet(t *testing.T) {
	testCases := []struct {
		ssl            ssl.Interface
		mcrt           *apisv1.ManagedCertificate
		wantCert       *compute.SslCertificate
		wantErr        error
		wantErrorEvent bool
	}{
		{
			ssl: withCert(nil, nil),
		},
		{
			ssl:  withCert(nil, nil),
			mcrt: managedcertificate.New(certId, domain).Build(),
		},
		{
			ssl:      withCert(nil, cert),
			wantCert: cert,
		},
		{
			ssl:      withCert(nil, cert),
			mcrt:     managedcertificate.New(certId, domain).Build(),
			wantCert: cert,
		},
		{
			ssl:     withCert(errGeneric, nil),
			wantErr: errGeneric,
		},
		{
			ssl:            withCert(errGeneric, nil),
			mcrt:           managedcertificate.New(certId, domain).Build(),
			wantErr:        errGeneric,
			wantErrorEvent: true,
		},
		{
			ssl:     withCert(errGeneric, cert),
			wantErr: errGeneric,
		},
		{
			ssl:            withCert(errGeneric, cert),
			mcrt:           managedcertificate.New(certId, domain).Build(),
			wantErr:        errGeneric,
			wantErrorEvent: true,
		},
	}

	for _, tc := range testCases {
		event := &event.Fake{}
		metrics := metrics.NewFake()
		sut := New(event, metrics, tc.ssl, state.NewFake())

		sslCert, err := sut.Get("", tc.mcrt)

		if err != tc.wantErr {
			t.Fatalf("err %#v, want %#v", err, tc.wantErr)
		} else if sslCert != tc.wantCert {
			t.Fatalf("cert: %#v, want %#v", sslCert, tc.wantCert)
		}

		oneErrorEvent := event.BackendErrorCnt == 1
		if tc.wantErrorEvent != oneErrorEvent {
			t.Fatalf("BackendError events generated: %d, want event: %t",
				event.BackendErrorCnt, tc.wantErrorEvent)
		}

		oneSslCertificateBackendErrorObserved := metrics.SslCertificateBackendErrorObserved == 1
		if tc.wantErrorEvent != oneSslCertificateBackendErrorObserved {
			t.Fatalf("Metric SslCertificateBackendError observed %d times",
				metrics.SslCertificateBackendErrorObserved)
		}
	}
}
