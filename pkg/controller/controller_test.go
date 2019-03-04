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

package controller

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"k8s.io/client-go/util/workqueue"

	api "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/networking.gke.io/v1beta1"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/fake"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/state"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/sync"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/types"
)

// Fake state
type fakeState struct {
	// Collection of ManagedCertificate ids
	ids []types.CertId
}

var _ state.StateIterator = &fakeState{}

func newFakeState(ids []types.CertId) *fakeState {
	return &fakeState{ids: ids}
}

func (f *fakeState) ForeachKey(fun func(id types.CertId)) {
	for _, id := range f.ids {
		fun(id)
	}
}

// Fake sync
type fakeSync struct {
	// Collection of ManagedCertificate ids
	ids []types.CertId
}

var _ sync.Sync = &fakeSync{}

func (f *fakeSync) ManagedCertificate(ctx context.Context, id types.CertId) error {
	f.ids = append(f.ids, id)
	return nil
}

// Fake queue
type fakeQueue struct {
	items []string
}

var _ workqueue.RateLimitingInterface = &fakeQueue{}

func (f *fakeQueue) Add(item interface{})                              {}
func (f *fakeQueue) Len() int                                          { return 0 }
func (f *fakeQueue) Get() (interface{}, bool)                          { return nil, false }
func (f *fakeQueue) Done(item interface{})                             {}
func (f *fakeQueue) ShutDown()                                         {}
func (f *fakeQueue) ShuttingDown() bool                                { return false }
func (f *fakeQueue) AddAfter(item interface{}, duration time.Duration) {}
func (f *fakeQueue) AddRateLimited(item interface{}) {
	f.items = append(f.items, item.(string))
}
func (f *fakeQueue) Forget(item interface{})          {}
func (f *fakeQueue) NumRequeues(item interface{}) int { return 0 }

func TestSynchronizeAllManagedCertificates(t *testing.T) {
	testCases := []struct {
		desc        string
		listerErr   error
		listerIds   []types.CertId
		stateIds    []types.CertId
		wantQueue   []string
		wantMetrics map[string]int
	}{
		{
			"State and lister empty",
			nil,
			nil,
			nil,
			nil,
			nil,
		},
		{
			"State two elements, lister one element",
			nil,
			[]types.CertId{types.NewCertId("default", "foo"), types.NewCertId("default", "bar")},
			[]types.CertId{types.NewCertId("default", "baz")},
			[]string{"default/foo", "default/bar"},
			map[string]int{"Active": 2},
		},
		{
			"State two elements, lister one element and fails",
			errors.New("generic error"),
			[]types.CertId{types.NewCertId("default", "foo"), types.NewCertId("default", "bar")},
			[]types.CertId{types.NewCertId("default", "baz")},
			nil,
			nil,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.desc, func(t *testing.T) {
			ctx := context.Background()

			var mcrts []*api.ManagedCertificate
			for _, id := range testCase.listerIds {
				mcrts = append(mcrts, fake.NewManagedCertificate(id, "example.com"))
			}

			metrics := fake.NewMetrics()
			queue := &fakeQueue{}
			sync := &fakeSync{}

			sut := &controller{
				lister:  fake.NewLister(testCase.listerErr, mcrts),
				metrics: metrics,
				queue:   queue,
				state:   newFakeState(testCase.stateIds),
				sync:    sync,
			}

			sut.synchronizeAllManagedCertificates(ctx)

			if !reflect.DeepEqual(testCase.stateIds, sync.ids) {
				t.Fatalf("Synced %v, want %v", sync.ids, testCase.stateIds)
			}

			if !reflect.DeepEqual(testCase.wantQueue, queue.items) {
				t.Fatalf("Enqueued %v, want: %v", queue.items, testCase.wantQueue)
			}

			if !reflect.DeepEqual(metrics.ManagedCertificatesStatuses, testCase.wantMetrics) {
				t.Fatalf("ManagedCertificate statuses: %v, want: %v",
					metrics.ManagedCertificatesStatuses, testCase.wantMetrics)
			}
		})
	}
}
