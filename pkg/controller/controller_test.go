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
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	netv1 "k8s.io/api/networking/v1"

	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/networking.gke.io/v1"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clients"
	clientsingress "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clients/ingress"
	clientsmcrt "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/clients/managedcertificate"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/liveness"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/metrics"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/state"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/sync"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/flags"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/testhelper/ingress"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/testhelper/managedcertificate"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/types"
)

type fakeSync struct {
	ingresses           map[types.Id]bool
	managedCertificates map[types.Id]bool
}

var _ sync.Interface = &fakeSync{}

func (f *fakeSync) Ingress(ctx context.Context, id types.Id) error {
	f.ingresses[id] = true
	return nil
}

func (f *fakeSync) ManagedCertificate(ctx context.Context, id types.Id) error {
	f.managedCertificates[id] = true
	return nil
}

// Controller maintains four workqueues which contain Ingresses and ManagedCertificates
// to be processed.
//
// The first pair of workqueues is used to handle resource creation, deletion or update.
// The second pair of workqueues is used to occasionally synchronize all resources.
//
// The test uses a fake `sync` component instead of one doing the real synchronization
// with external state and user intent. Fake sync counts all the resources it sees.
// The aim of the test is to make sure that all expected resources were queued and
// delivered to sync.
func TestController(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		description                  string
		ingresses                    []types.Id
		managedCertificatesInCluster []types.Id
		managedCertificatesInState   []types.Id

		wantIngresses           map[types.Id]bool
		wantManagedCertificates map[types.Id]bool
		wantMetrics             map[string]int
	}{
		{
			description: "No items",
		},
		{
			description: "Items",
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
			wantIngresses: map[types.Id]bool{
				types.NewId("default", "a1"): true,
				types.NewId("default", "a2"): true,
			},
			wantManagedCertificates: map[types.Id]bool{
				types.NewId("default", "b1"): true,
				types.NewId("default", "b2"): true,
				types.NewId("default", "b3"): true,
			},
			wantMetrics: map[string]int{"Active": 2},
		},
	}

	flags.Register()
	for i, testCase := range testCases {
		i, testCase := i, testCase
		t.Run(testCase.description, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

			var ingresses []*netv1.Ingress
			for _, id := range testCase.ingresses {
				ingresses = append(ingresses, ingress.New(id))
			}

			var mcrts []*v1.ManagedCertificate
			for _, id := range testCase.managedCertificatesInCluster {
				mcrts = append(mcrts, managedcertificate.
					New(id, "example.com").
					WithStatus("Active", "Active").
					Build())
			}

			stateEntries := make(map[types.Id]state.Entry, 0)
			for _, id := range testCase.managedCertificatesInState {
				stateEntries[id] = state.Entry{}
			}

			healthCheck := liveness.NewHealthCheck(time.Second,
				5*time.Second, 5*time.Second)
			healthCheck.StartServing(fmt.Sprintf(":%d", 27500+i), "/health-check")

			metrics := metrics.NewFake()
			sync := &fakeSync{
				ingresses:           make(map[types.Id]bool),
				managedCertificates: make(map[types.Id]bool),
			}
			ctrl := New(ctx, &params{
				clients: &clients.Clients{
					Ingress:            clientsingress.NewFake(ingresses),
					ManagedCertificate: clientsmcrt.NewFake(mcrts),
				},
				metrics:        metrics,
				healthCheck:    healthCheck,
				resyncInterval: time.Minute,
				state:          state.NewFakeWithEntries(stateEntries),
				sync:           sync,
			})

			// Trigger resources queuing.
			go ctrl.Run(ctx)

			// Loop until all expected resources are processed.
			go func() {
				for len(sync.ingresses) < len(testCase.wantIngresses) ||
					len(sync.managedCertificates) < len(testCase.wantManagedCertificates) {

					t.Logf("%d/%d Ingresses, %d/%d ManagedCertificates synchronized",
						len(sync.ingresses), len(testCase.wantIngresses),
						len(sync.managedCertificates), len(testCase.wantManagedCertificates))
					time.Sleep(500 * time.Millisecond)
				}

				cancel()
			}()

			<-ctx.Done()

			if diff := cmp.Diff(testCase.wantIngresses, sync.ingresses, cmpopts.EquateEmpty()); diff != "" {
				t.Fatalf("Diff Ingresses (-want, +got): %s", diff)
			}

			if diff := cmp.Diff(testCase.wantManagedCertificates, sync.managedCertificates, cmpopts.EquateEmpty()); diff != "" {
				t.Fatalf("Diff ManagedCertificates (-want, +got): %s", diff)
			}

			if diff := cmp.Diff(testCase.wantMetrics, metrics.ManagedCertificatesStatuses); diff != "" {
				t.Fatalf("Diff metrics (-want, +got): %s", diff)
			}
		})
	}
}
