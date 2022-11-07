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

package managedcertificate

import (
	"context"
	"encoding/json"

	"github.com/evanphx/json-patch"
	"k8s.io/client-go/util/workqueue"

	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/networking.gke.io/v1"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/errors"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/types"
)

type Fake struct {
	managedCertificates []*v1.ManagedCertificate
}

var _ Interface = (*Fake)(nil)

func NewFake(managedCertificates []*v1.ManagedCertificate) *Fake {
	return &Fake{managedCertificates: managedCertificates}
}

func (f *Fake) Get(id types.Id) (*v1.ManagedCertificate, error) {
	for _, cert := range f.managedCertificates {
		if cert.Namespace == id.Namespace && cert.Name == id.Name {
			return cert, nil
		}
	}

	return nil, errors.NotFound
}

func (f *Fake) HasSynced() bool {
	return true
}

func (f *Fake) List() ([]*v1.ManagedCertificate, error) {
	return f.managedCertificates, nil
}

func (f *Fake) Patch(ctx context.Context, id types.Id, diff []byte) error {
	for i, cert := range f.managedCertificates {
		if cert.Namespace == id.Namespace && cert.Name == id.Name {
			mcrtBytes, err := json.Marshal(f.managedCertificates[i])
			if err != nil {
				return err
			}
			mcrtBytes, err = jsonpatch.MergePatch(mcrtBytes, diff)
			if err != nil {
				return err
			}
			err = json.Unmarshal(mcrtBytes, f.managedCertificates[i])
			if err != nil {
				return err
			}
			return nil
		}
	}
	return errors.NotFound
}

func (f *Fake) Run(ctx context.Context, queue workqueue.RateLimitingInterface) {}
