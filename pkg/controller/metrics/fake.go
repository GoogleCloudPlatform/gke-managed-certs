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

package metrics

import (
	"time"
)

type Fake struct {
	ManagedCertificatesStatuses           map[string]int
	SslCertificateBackendErrorObserved    int
	SslCertificateQuotaErrorObserved      int
	SslCertificateBindingLatencyObserved  int
	SslCertificateCreationLatencyObserved int
}

var _ Interface = &Fake{}

func NewFake() *Fake {
	return &Fake{}
}

func (f *Fake) Start(address string) {}

func (f *Fake) ObserveManagedCertificatesStatuses(statuses map[string]int) {
	f.ManagedCertificatesStatuses = statuses
}

func (f *Fake) ObserveSslCertificateBackendError() {
	f.SslCertificateBackendErrorObserved++
}

func (f *Fake) ObserveSslCertificateQuotaError() {
	f.SslCertificateQuotaErrorObserved++
}

func (f *Fake) ObserveSslCertificateBindingLatency(creationTime time.Time) {
	f.SslCertificateBindingLatencyObserved++
}

func (f *Fake) ObserveSslCertificateCreationLatency(creationTime time.Time) {
	f.SslCertificateCreationLatencyObserved++
}
