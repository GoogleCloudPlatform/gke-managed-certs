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
	networkingv1beta1 "k8s.io/api/networking/v1beta1"

	apisv1 "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/networking.gke.io/v1"
)

type Fake struct {
	BackendErrorCnt        int
	CreateCnt              int
	DeleteCnt              int
	MissingCertificateCnt  int
	TooManyCertificatesCnt int
}

var _ Interface = (*Fake)(nil)

func (f *Fake) BackendError(mcrt apisv1.ManagedCertificate, err error) {
	f.BackendErrorCnt++
}

func (f *Fake) Create(mcrt apisv1.ManagedCertificate, sslCertificateName string) {
	f.CreateCnt++
}

func (f *Fake) Delete(mcrt apisv1.ManagedCertificate, sslCertificateName string) {
	f.DeleteCnt++
}

func (f *Fake) MissingCertificate(ingress networkingv1beta1.Ingress, mcrtName string) {
	f.MissingCertificateCnt++
}

func (f *Fake) TooManyCertificates(mcrt apisv1.ManagedCertificate, err error) {
	f.TooManyCertificatesCnt++
}
