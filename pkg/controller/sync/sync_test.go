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
	"k8s.io/apimachinery/pkg/runtime/schema"
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

var genericError = errors.New("generic error")
var googleNotFound = &googleapi.Error{
	Code: 404,
}
var k8sNotFound = k8s_errors.NewNotFound(schema.GroupResource{
	Group:    "test_group",
	Resource: "test_resource",
}, "test_name")

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

var listerFailsGenericErr = newLister(genericError, nil)
var listerFailsNotFound = newLister(k8sNotFound, nil)
var listerSuccess = newLister(nil, mcrt)

var randomFailsGenericErr = newRandom(genericError, "")
var randomSuccess = newRandom(nil, sslCertificateName)

func empty() *fakeState {
	return newState("", "", "")
}
func withEntry() *fakeState {
	return newState(namespace, name, sslCertificateName)
}

type in struct {
	lister       fakeLister
	random       fakeRandom
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
			lister: listerFailsGenericErr,
			random: randomSuccess,
			state:  empty(),
		}, false, false, genericError,
	},
	{
		"Lister fails with generic error, entry in state",
		in{
			lister: listerFailsGenericErr,
			random: randomSuccess,
			state:  withEntry(),
		}, true, false, genericError,
	},
	{
		"Lister fails with not found, state is empty",
		in{
			lister: listerFailsNotFound,
			random: randomSuccess,
			state:  empty(),
		}, false, false, nil,
	},
	{
		"Lister fails with not found, entry in state, success to delete ssl cert",
		in{
			lister: listerFailsNotFound,
			random: randomSuccess,
			state:  withEntry(),
		}, false, false, nil,
	},
	{
		"Lister fails with not found, entry in state, ssl cert already deleted",
		in{
			lister:       listerFailsNotFound,
			random:       randomSuccess,
			state:        withEntry(),
			sslDeleteErr: []error{googleNotFound},
		}, false, false, nil,
	},
	{
		"Lister fails with not found, entry in state, ssl cert fails to delete",
		in{
			lister:       listerFailsNotFound,
			random:       randomSuccess,
			state:        withEntry(),
			sslDeleteErr: []error{genericError},
		}, true, false, genericError,
	},
	{
		"Lister success, state empty",
		in{
			lister: listerSuccess,
			random: randomSuccess,
			state:  empty(),
		}, true, true, nil,
	},
	{
		"Lister success, entry in state",
		in{
			lister: listerSuccess,
			random: randomSuccess,
			state:  withEntry(),
		}, true, true, nil,
	},
	{
		"Lister success, state empty, random fails",
		in{
			lister: listerSuccess,
			random: randomFailsGenericErr,
			state:  empty(),
		}, false, false, genericError,
	},
	{
		"Lister success, entry in state, random fails",
		in{
			lister: listerSuccess,
			random: randomFailsGenericErr,
			state:  withEntry(),
		}, true, true, nil,
	},
	{
		"Lister success, state empty, ssl cert exists fails",
		in{
			lister:       listerSuccess,
			random:       randomSuccess,
			state:        empty(),
			sslExistsErr: []error{genericError},
		}, true, false, genericError,
	},
	{
		"Lister success, entry in state, ssl cert exists fails",
		in{
			lister:       listerSuccess,
			random:       randomSuccess,
			state:        withEntry(),
			sslExistsErr: []error{genericError},
		}, true, false, genericError,
	},
	{
		"Lister success, state empty, ssl cert create fails",
		in{
			lister:       listerSuccess,
			random:       randomSuccess,
			state:        empty(),
			sslCreateErr: []error{genericError},
		}, true, false, genericError,
	},
	{
		"Lister success, entry in state, ssl cert create fails",
		in{
			lister:       listerSuccess,
			random:       randomSuccess,
			state:        withEntry(),
			sslCreateErr: []error{genericError},
		}, true, false, genericError,
	},
	{
		"Lister success, state empty, ssl cert get fails",
		in{
			lister:    listerSuccess,
			random:    randomSuccess,
			state:     empty(),
			sslGetErr: []error{genericError},
		}, true, false, genericError,
	},
	{
		"Lister success, entry in state, ssl cert get fails",
		in{
			lister:    listerSuccess,
			random:    randomSuccess,
			state:     withEntry(),
			sslGetErr: []error{genericError},
		}, true, false, genericError,
	},
	{
		"Lister success, state empty, ssl cert get fails",
		in{
			lister:    listerSuccess,
			random:    randomSuccess,
			state:     empty(),
			mcrt:      mcrt,
			sslGetErr: []error{genericError},
		}, true, false, genericError,
	},
	{
		"Lister success, entry in state, ssl cert get fails",
		in{
			lister:    listerSuccess,
			random:    randomSuccess,
			state:     withEntry(),
			mcrt:      mcrt,
			sslGetErr: []error{genericError},
		}, true, false, genericError,
	},
	{
		"Lister success, entry in state, certs mismatch",
		in{
			lister: listerSuccess,
			random: randomSuccess,
			state:  withEntry(),
			mcrt:   differentMcrt,
		}, true, true, nil,
	},
	{
		"Lister success, entry in state, certs mismatch but ssl cert already deleted",
		in{
			lister:       listerSuccess,
			random:       randomSuccess,
			state:        withEntry(),
			mcrt:         differentMcrt,
			sslDeleteErr: []error{googleNotFound},
		}, true, true, nil,
	},
	{
		"Lister success, entry in state, certs mismatch and deletion fails",
		in{
			lister:       listerSuccess,
			random:       randomSuccess,
			state:        withEntry(),
			mcrt:         differentMcrt,
			sslDeleteErr: []error{genericError},
		}, true, false, genericError,
	},
	{
		"Lister success, entry in state, certs mismatch, ssl cert creation fails",
		in{
			lister:       listerSuccess,
			random:       randomSuccess,
			state:        withEntry(),
			mcrt:         differentMcrt,
			sslCreateErr: []error{genericError},
		}, true, false, genericError,
	},
	{
		"Lister success, entry in state, certs mismatch but ssl cert already deleted, ssl cert creation fails",
		in{
			lister:       listerSuccess,
			random:       randomSuccess,
			state:        withEntry(),
			mcrt:         differentMcrt,
			sslCreateErr: []error{genericError},
			sslDeleteErr: []error{googleNotFound},
		}, true, false, genericError,
	},
	{
		"Lister success, entry in state, certs mismatch, ssl cert get fails",
		in{
			lister:    listerSuccess,
			random:    randomSuccess,
			state:     withEntry(),
			mcrt:      differentMcrt,
			sslGetErr: []error{nil, genericError},
		}, true, false, genericError,
	},
	{
		"Lister success, entry in state, certs mismatch but ssl cert already deleted, ssl cert get fails",
		in{
			lister:       listerSuccess,
			random:       randomSuccess,
			state:        withEntry(),
			mcrt:         differentMcrt,
			sslDeleteErr: []error{googleNotFound},
			sslGetErr:    []error{nil, genericError},
		}, true, false, genericError,
	},
}

func TestManagedCertificate(t *testing.T) {
	for _, testCase := range testCases {
		t.Run(testCase.desc, func(t *testing.T) {
			clientset := newClientset()
			updateCalled := false
			clientset.AddReactor("update", "*", buildUpdateFunc(&updateCalled))

			ssl := newSsl(sslCertificateName, testCase.in.mcrt, testCase.in.sslCreateErr, testCase.in.sslDeleteErr, testCase.in.sslExistsErr, testCase.in.sslGetErr)

			sut := New(clientset, testCase.in.lister, testCase.in.random, ssl, testCase.in.state)

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
