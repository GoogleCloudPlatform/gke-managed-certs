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

package sync

import (
	"errors"
	"testing"

	"google.golang.org/api/googleapi"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	cgo_testing "k8s.io/client-go/testing"

	api "github.com/GoogleCloudPlatform/gke-managed-certs/pkg/apis/gke.googleapis.com/v1alpha1"
)

const (
	namespace          = "foo"
	name               = "bar"
	sslCertificateName = "baz"
)

func buildUpdateFunc(updateCalled *bool) cgo_testing.ReactionFunc {
	return cgo_testing.ReactionFunc(func(action cgo_testing.Action) (bool, runtime.Object, error) {
		*updateCalled = true
		return true, nil, nil
	})
}

var normalError = errors.New("normal error")
var googleNotFound = &googleapi.Error{
	Code: 404,
}
var k8sNotFound = &k8s_errors.StatusError{
	metav1.Status{
		Code: 404,
	},
}

var mcrt = &api.ManagedCertificate{
	ObjectMeta: metav1.ObjectMeta{
		Namespace: namespace,
		Name:      name,
	},
	Spec: api.ManagedCertificateSpec{
		Domains: []string{"example.com"},
	},
}
var differentMcrt = &api.ManagedCertificate{
	ObjectMeta: metav1.ObjectMeta{
		Namespace: namespace,
		Name:      name,
	},
	Spec: api.ManagedCertificateSpec{
		Domains: []string{"example2.com"},
	},
}

var failsNormalErr = newLister(normalError, nil)
var failsNotFound = newLister(k8sNotFound, nil)
var success = newLister(nil, mcrt)

func empty() *fakeState {
	return newState("", "", "")
}
func withEntry() *fakeState {
	return newState(namespace, name, sslCertificateName)
}

type in struct {
	lister       fakeLister
	state        *fakeState
	mcrt         *api.ManagedCertificate
	sslCreateErr []error
	sslDeleteErr []error
	sslExistsErr []error
	sslGetErr    []error
}

var testCases = []struct {
	desc            string
	in              in
	outEntryInState bool
	outUpdateCalled bool
	outErr          error
}{
	{
		"Lister fails with generic error, state is empty",
		in{
			lister: failsNormalErr,
			state:  empty(),
		}, false, false, normalError,
	},
	{
		"Lister fails with generic error, entry in state",
		in{
			lister: failsNormalErr,
			state:  withEntry(),
		}, true, false, normalError,
	},
	{
		"Lister fails with not found, state is empty",
		in{
			lister: failsNotFound,
			state:  empty(),
		}, false, false, nil,
	},
	{
		"Lister fails with not found, entry in state, success to delete ssl cert",
		in{
			lister: failsNotFound,
			state:  withEntry(),
		}, false, false, nil,
	},
	{
		"Lister fails with not found, entry in state, ssl cert already deleted",
		in{
			lister:       failsNotFound,
			state:        withEntry(),
			sslDeleteErr: []error{googleNotFound},
		}, false, false, nil,
	},
	{
		"Lister fails with not found, entry in state, ssl cert fails to delete",
		in{
			lister:       failsNotFound,
			state:        withEntry(),
			sslDeleteErr: []error{normalError},
		}, true, false, normalError,
	},
	{
		"Lister success, state empty",
		in{
			lister: success,
			state:  empty(),
		}, true, true, nil,
	},
	{
		"Lister success, entry in state",
		in{
			lister: success,
			state:  withEntry(),
		}, true, true, nil,
	},
	{
		"Lister success, state empty, ssl cert exists fails",
		in{
			lister:       success,
			state:        empty(),
			sslExistsErr: []error{normalError},
		}, true, false, normalError,
	},
	{
		"Lister success, entry in state, ssl cert exists fails",
		in{
			lister:       success,
			state:        withEntry(),
			sslExistsErr: []error{normalError},
		}, true, false, normalError,
	},
	{
		"Lister success, state empty, ssl cert create fails",
		in{
			lister:       success,
			state:        empty(),
			sslCreateErr: []error{normalError},
		}, true, false, normalError,
	},
	{
		"Lister success, entry in state, ssl cert create fails",
		in{
			lister:       success,
			state:        withEntry(),
			sslCreateErr: []error{normalError},
		}, true, false, normalError,
	},
	{
		"Lister success, state empty, ssl cert get fails",
		in{
			lister:    success,
			state:     empty(),
			sslGetErr: []error{normalError},
		}, true, false, normalError,
	},
	{
		"Lister success, entry in state, ssl cert get fails",
		in{
			lister:    success,
			state:     withEntry(),
			sslGetErr: []error{normalError},
		}, true, false, normalError,
	},
	{
		"Lister success, state empty, ssl cert get fails",
		in{
			lister:    success,
			state:     empty(),
			mcrt:      mcrt,
			sslGetErr: []error{normalError},
		}, true, false, normalError,
	},
	{
		"Lister success, entry in state, ssl cert get fails",
		in{
			lister:    success,
			state:     withEntry(),
			mcrt:      mcrt,
			sslGetErr: []error{normalError},
		}, true, false, normalError,
	},
	{
		"Lister success, entry in state, certs mismatch",
		in{
			lister: success,
			state:  withEntry(),
			mcrt:   differentMcrt,
		}, true, true, nil,
	},
	{
		"Lister success, entry in state, certs mismatch but ssl cert already deleted",
		in{
			lister:       success,
			state:        withEntry(),
			mcrt:         differentMcrt,
			sslDeleteErr: []error{googleNotFound},
		}, true, true, nil,
	},
	{
		"Lister success, entry in state, certs mismatch and deletion fails",
		in{
			lister:       success,
			state:        withEntry(),
			mcrt:         differentMcrt,
			sslDeleteErr: []error{normalError},
		}, true, false, normalError,
	},
	{
		"Lister success, entry in state, certs mismatch, ssl cert creation fails",
		in{
			lister:       success,
			state:        withEntry(),
			mcrt:         differentMcrt,
			sslCreateErr: []error{normalError},
		}, true, false, normalError,
	},
	{
		"Lister success, entry in state, certs mismatch but ssl cert already deleted, ssl cert creation fails",
		in{
			lister:       success,
			state:        withEntry(),
			mcrt:         differentMcrt,
			sslCreateErr: []error{normalError},
			sslDeleteErr: []error{googleNotFound},
		}, true, false, normalError,
	},
	{
		"Lister success, entry in state, certs mismatch, ssl cert get fails",
		in{
			lister:    success,
			state:     withEntry(),
			mcrt:      differentMcrt,
			sslGetErr: []error{nil, normalError},
		}, true, false, normalError,
	},
	{
		"Lister success, entry in state, certs mismatch but ssl cert already deleted, ssl cert get fails",
		in{
			lister:       success,
			state:        withEntry(),
			mcrt:         differentMcrt,
			sslDeleteErr: []error{googleNotFound},
			sslGetErr:    []error{nil, normalError},
		}, true, false, normalError,
	},
}

func TestManagedCertificate(t *testing.T) {
	for _, testCase := range testCases {
		t.Run(testCase.desc, func(t *testing.T) {
			clientset := newClientset()
			updateCalled := false
			clientset.AddReactor("update", "*", buildUpdateFunc(&updateCalled))

			ssl := newSsl(sslCertificateName, testCase.in.mcrt, testCase.in.sslCreateErr, testCase.in.sslDeleteErr, testCase.in.sslExistsErr, testCase.in.sslGetErr)

			sut := New(clientset, testCase.in.lister, ssl, testCase.in.state)

			err := sut.ManagedCertificate(namespace, name)

			if _, e := testCase.in.state.Get(namespace, name); e != testCase.outEntryInState {
				t.Errorf("Entry in state %t, want %t", e, testCase.outEntryInState)
			}

			if testCase.outUpdateCalled != updateCalled {
				t.Errorf("Update called %t, want %t", updateCalled, testCase.outUpdateCalled)
			}

			if err != testCase.outErr {
				t.Errorf("Have error: %v, want: %v", err, testCase.outErr)
			}
		})
	}
}
