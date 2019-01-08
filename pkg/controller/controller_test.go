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
	"errors"
	"reflect"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"

	api "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/gke.googleapis.com/v1alpha1"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/fake"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/state"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/controller/sync"
)

type pair struct {
	first  string
	second string
}

// Fake state
type fakeState struct {
	// Collection of namespace, name pairs
	managedCertificates []pair
}

var _ state.State = &fakeState{}

func newFakeState(managedCertificates []pair) *fakeState {
	return &fakeState{
		managedCertificates: managedCertificates,
	}
}

func (f *fakeState) Delete(namespace, name string) {}

func (f *fakeState) ForeachKey(fun func(namespace, name string)) {
	for _, m := range f.managedCertificates {
		fun(m.first, m.second)
	}
}

func (f *fakeState) GetSslCertificateName(namespace, name string) (string, bool) {
	return "", false
}

func (f *fakeState) IsSslCertificateCreationReported(namespace, name string) (bool, bool) {
	return false, false
}

func (f *fakeState) SetSslCertificateCreationReported(namespace, name string) {}

func (f *fakeState) SetSslCertificateName(namespace, name, sslCertificateName string) {}

// Fake sync
type fakeSync struct {
	// Collection of namespace, name pairs
	managedCertificates []pair
}

var _ sync.Sync = &fakeSync{}

func (f *fakeSync) ManagedCertificate(namespace, name string) error {
	f.managedCertificates = append(f.managedCertificates, pair{first: namespace, second: name})
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
		desc      string
		listerErr error
		// Collection of namespace, name pairs
		managedCertificatesLister []pair
		managedCertificatesState  []pair
		wantQueue                 []string
		wantMetrics               map[string]int
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
			[]pair{pair{"default", "foo"}, pair{"default", "bar"}},
			[]pair{pair{"default", "baz"}},
			[]string{"default/foo", "default/bar"},
			map[string]int{"Active": 2},
		},
		{
			"State two elements, lister one element and fails",
			errors.New("test error"),
			[]pair{pair{"default", "foo"}, pair{"default", "bar"}},
			[]pair{pair{"default", "baz"}},
			nil,
			nil,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.desc, func(t *testing.T) {
			var mcrts []*api.ManagedCertificate
			for _, mcrt := range testCase.managedCertificatesLister {
				mcrts = append(mcrts, &api.ManagedCertificate{
					ObjectMeta: metav1.ObjectMeta{
						CreationTimestamp: metav1.Now().Rfc3339Copy(),
						Namespace:         mcrt.first,
						Name:              mcrt.second,
					},
					Spec: api.ManagedCertificateSpec{
						Domains: []string{"example.com"},
					},
					Status: api.ManagedCertificateStatus{
						CertificateStatus: "Active",
					},
				})
			}

			metrics := fake.NewMetrics()
			queue := &fakeQueue{}
			sync := &fakeSync{}

			sut := &controller{
				lister:  fake.NewLister(testCase.listerErr, mcrts),
				metrics: metrics,
				queue:   queue,
				state:   newFakeState(testCase.managedCertificatesState),
				sync:    sync,
			}

			sut.synchronizeAllManagedCertificates()

			if !reflect.DeepEqual(testCase.managedCertificatesState, sync.managedCertificates) {
				t.Fatalf("Synced %v, want %v", sync.managedCertificates, testCase.managedCertificatesState)
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
