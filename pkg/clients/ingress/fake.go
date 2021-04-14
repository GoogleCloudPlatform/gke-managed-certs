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

package ingress

import (
	"context"

	"k8s.io/api/networking/v1"
	"k8s.io/client-go/util/workqueue"

	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/errors"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/types"
)

type Fake struct {
	ingresses []*v1.Ingress
}

var _ Interface = (*Fake)(nil)

func NewFake(ingresses []*v1.Ingress) *Fake {
	return &Fake{ingresses: ingresses}
}

func (f *Fake) Get(id types.Id) (*v1.Ingress, error) {
	for _, ingress := range f.ingresses {
		if ingress.Namespace == id.Namespace && ingress.Name == id.Name {
			return ingress, nil
		}
	}

	return nil, errors.NotFound
}

func (f *Fake) HasSynced() bool {
	return true
}

func (f *Fake) List() ([]*v1.Ingress, error) {
	return f.ingresses, nil
}

func (f *Fake) Update(ctx context.Context, ingress *v1.Ingress) error {
	for i, ing := range f.ingresses {
		if ing.Namespace == ingress.Namespace && ing.Name == ingress.Name {
			f.ingresses[i] = ingress
			return nil
		}
	}

	return errors.NotFound
}

func (f *Fake) Run(ctx context.Context, queue workqueue.RateLimitingInterface) {}
