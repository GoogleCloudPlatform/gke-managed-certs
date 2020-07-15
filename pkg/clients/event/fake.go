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

package event

import (
	apisv1beta2 "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/networking.gke.io/v1beta2"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
)

type FakeEvent struct {
	BackendErrorCnt int
	CreateCnt       int
	DeleteCnt       int
	MissingCnt      int
	TooManyCnt      int
}

var _ Event = (*FakeEvent)(nil)

func (f *FakeEvent) BackendError(mcrt apisv1beta2.ManagedCertificate, err error) {
	f.BackendErrorCnt++
}

func (f *FakeEvent) Create(mcrt apisv1beta2.ManagedCertificate, sslCertificateName string) {
	f.CreateCnt++
}

func (f *FakeEvent) Delete(mcrt apisv1beta2.ManagedCertificate, sslCertificateName string) {
	f.DeleteCnt++
}

func (f *FakeEvent) MissingCertificate(ingress extv1beta1.Ingress, mcrtName string) {
	f.MissingCnt++
}

func (f *FakeEvent) TooManyCertificates(mcrt apisv1beta2.ManagedCertificate, err error) {
	f.TooManyCnt++
}
