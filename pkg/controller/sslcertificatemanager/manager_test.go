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

package sslcertificatemanager

import (
	"errors"
	"testing"

	api "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/networking.gke.io/v1beta1"
	compute "google.golang.org/api/compute/v0.beta"
	"google.golang.org/api/googleapi"

	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clients/event"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clients/ssl"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/fake"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/types"
)

var (
	domain = "foo.com"
	certId = types.NewCertId("default", "bar")
)

type fakeSsl struct {
	err            error
	exists         bool
	sslCertificate *compute.SslCertificate
}

var _ ssl.Ssl = (*fakeSsl)(nil)

func (f fakeSsl) Create(name string, domains []string) error {
	return f.err
}

func (f fakeSsl) Delete(name string) error {
	return f.err
}

func (f fakeSsl) Exists(name string) (bool, error) {
	return f.exists, f.err
}

func (f fakeSsl) Get(name string) (*compute.SslCertificate, error) {
	return f.sslCertificate, f.err
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

func (f *fakeEvent) BackendError(mcrt api.ManagedCertificate, err error) {
	f.backendErrorCnt++
}

func (f *fakeEvent) Create(mcrt api.ManagedCertificate, sslCertificateName string) {
	f.createCnt++
}

func (f *fakeEvent) Delete(mcrt api.ManagedCertificate, sslCertificateName string) {
	f.deleteCnt++
}

func (f *fakeEvent) TooManyCertificates(mcrt api.ManagedCertificate, err error) {
	f.tooManyCnt++
}

var genericErr = errors.New("generic error")
var quotaExceededErr = &googleapi.Error{
	Code: 403,
	Errors: []googleapi.ErrorItem{
		googleapi.ErrorItem{
			Reason: "quotaExceeded",
		},
	},
}
var notFoundErr = &googleapi.Error{
	Code: 404,
}
var cert = &compute.SslCertificate{}

func TestCreate(t *testing.T) {
	testCases := []struct {
		ssl                   ssl.Ssl
		wantErr               error
		wantTooManyCertsEvent bool
		wantBackendErrorEvent bool
		wantCreateEvent       bool
	}{
		{
			ssl:             withErr(nil),
			wantCreateEvent: true,
		},
		{
			ssl:                   withErr(quotaExceededErr),
			wantErr:               quotaExceededErr,
			wantTooManyCertsEvent: true,
		},
		{
			ssl:                   withErr(genericErr),
			wantErr:               genericErr,
			wantBackendErrorEvent: true,
		},
	}

	for _, tc := range testCases {
		event := &fakeEvent{0, 0, 0, 0}
		metrics := fake.NewMetrics()
		sut := New(event, metrics, tc.ssl)

		err := sut.Create("", *fake.NewManagedCertificate(certId, domain))

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
		mcrt            *api.ManagedCertificate
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
			ssl:     withErr(genericErr),
			wantErr: genericErr,
		},
		{
			ssl:            withErr(genericErr),
			mcrt:           fake.NewManagedCertificate(certId, domain),
			wantErr:        genericErr,
			wantErrorEvent: true,
		},
		{
			ssl: withErr(notFoundErr),
		},
		{
			ssl:  withErr(notFoundErr),
			mcrt: fake.NewManagedCertificate(certId, domain),
		},
	}

	for _, tc := range testCases {
		event := &fakeEvent{0, 0, 0, 0}
		metrics := fake.NewMetrics()
		sut := New(event, metrics, tc.ssl)

		err := sut.Delete("", tc.mcrt)

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
		mcrt           *api.ManagedCertificate
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
			ssl:     withExists(genericErr, false),
			wantErr: genericErr,
		},
		{
			ssl:            withExists(genericErr, false),
			mcrt:           fake.NewManagedCertificate(certId, domain),
			wantErr:        genericErr,
			wantErrorEvent: true,
		},
		{
			ssl:     withExists(genericErr, true),
			wantErr: genericErr,
		},
		{
			ssl:            withExists(genericErr, true),
			mcrt:           fake.NewManagedCertificate(certId, domain),
			wantErr:        genericErr,
			wantErrorEvent: true,
		},
	}

	for _, tc := range testCases {
		event := &fakeEvent{0, 0, 0, 0}
		metrics := fake.NewMetrics()
		sut := New(event, metrics, tc.ssl)

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
		mcrt           *api.ManagedCertificate
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
			ssl:     withCert(genericErr, nil),
			wantErr: genericErr,
		},
		{
			ssl:            withCert(genericErr, nil),
			mcrt:           fake.NewManagedCertificate(certId, domain),
			wantErr:        genericErr,
			wantErrorEvent: true,
		},
		{
			ssl:     withCert(genericErr, cert),
			wantErr: genericErr,
		},
		{
			ssl:            withCert(genericErr, cert),
			mcrt:           fake.NewManagedCertificate(certId, domain),
			wantErr:        genericErr,
			wantErrorEvent: true,
		},
	}

	for _, tc := range testCases {
		event := &fakeEvent{0, 0, 0, 0}
		metrics := fake.NewMetrics()
		sut := New(event, metrics, tc.ssl)

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
