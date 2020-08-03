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

package controller

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	apiv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/client-go/util/workqueue"

	apisv1 "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/networking.gke.io/v1"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clients"
	clientsingress "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clients/ingress"
	clientsmcrt "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clients/managedcertificate"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/metrics"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/state"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/sync"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/testhelper/ingress"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/testhelper/managedcertificate"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/types"
)

type fakeSync struct {
	ingresses           []types.Id
	managedCertificates []types.Id
}

var _ sync.Interface = &fakeSync{}

func (f *fakeSync) Ingress(ctx context.Context, id types.Id) error {
	f.ingresses = append(f.ingresses, id)
	return nil
}

func (f *fakeSync) ManagedCertificate(ctx context.Context, id types.Id) error {
	f.managedCertificates = append(f.managedCertificates, id)
	return nil
}

func TestController(t *testing.T) {
	testCases := map[string]struct {
		ingresses                    []types.Id
		managedCertificatesInCluster []types.Id
		managedCertificatesInState   []types.Id

		wantIngresses           []types.Id
		wantManagedCertificates []types.Id
		wantMetrics             map[string]int
	}{
		"No items": {},
		"Items": {
			ingresses: []types.Id{
				types.NewId("default", "a1"),
				types.NewId("default", "a2"),
			},
			managedCertificatesInCluster: []types.Id{
				types.NewId("default", "b1"),
				types.NewId("default", "b2"),
			},
			managedCertificatesInState: []types.Id{
				types.NewId("default", "b1"),
				types.NewId("default", "b3"),
			},
			wantIngresses: []types.Id{
				types.NewId("default", "a1"),
				types.NewId("default", "a2"),
			},
			wantManagedCertificates: []types.Id{
				types.NewId("default", "b1"),
				types.NewId("default", "b2"),
				types.NewId("default", "b3"),
			},
			wantMetrics: map[string]int{"Active": 2},
		},
	}

	for description, testCase := range testCases {
		t.Run(description, func(t *testing.T) {
			ctx := context.Background()
			ctx, cancel := context.WithCancel(ctx)

			var ingresses []*apiv1beta1.Ingress
			for _, id := range testCase.ingresses {
				ingresses = append(ingresses, ingress.New(id, "", ""))
			}

			var mcrts []*apisv1.ManagedCertificate
			for _, id := range testCase.managedCertificatesInCluster {
				mcrts = append(mcrts, managedcertificate.New(id, "example.com").WithStatus("Active", "Active").Build())
			}

			stateEntries := make(map[types.Id]state.Entry, 0)
			for _, id := range testCase.managedCertificatesInState {
				stateEntries[id] = state.Entry{}
			}

			metrics := metrics.NewFake()
			sync := &fakeSync{}

			ctrl := &controller{
				clients: &clients.Clients{
					Ingress:            clientsingress.NewFake(ingresses),
					ManagedCertificate: clientsmcrt.NewFake(mcrts),
				},
				metrics:                 metrics,
				ingressQueue:            workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "ingressQueue"),
				managedCertificateQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "managedCertificateQueue"),
				state:                   state.NewFakeWithEntries(stateEntries),
				sync:                    sync,
			}

			go ctrl.Run(ctx)

			go func() {
				for len(sync.ingresses) < len(testCase.wantIngresses) || len(sync.managedCertificates) < len(testCase.wantManagedCertificates) {
				}

				cancel()
			}()

			<-ctx.Done()

			if diff := cmp.Diff(testCase.wantIngresses, sync.ingresses); diff != "" {
				t.Fatalf("Diff Ingresses (-want, +got): %s", diff)
			}

			if diff := cmp.Diff(testCase.wantManagedCertificates, sync.managedCertificates); diff != "" {
				t.Fatalf("Diff ManagedCertificates (-want, +got): %s", diff)
			}

			if diff := cmp.Diff(testCase.wantMetrics, metrics.ManagedCertificatesStatuses); diff != "" {
				t.Fatalf("Diff metrics (-want, +got): %s", diff)
			}
		})
	}
}
