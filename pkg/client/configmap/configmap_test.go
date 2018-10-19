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

package configmap

import (
	"errors"
	"testing"

	api "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/typed/core/v1/fake"
	cgo_testing "k8s.io/client-go/testing"
)

const (
	namespace = "default"
	resource  = "configmaps"
)

var normal = errors.New("normal error")
var k8sNotFound = &k8s_errors.StatusError{
	metav1.Status{
		Code: 404,
	},
}

func buildUpdateFunc(err error) cgo_testing.ReactionFunc {
	return cgo_testing.ReactionFunc(func(action cgo_testing.Action) (bool, runtime.Object, error) {
		return true, nil, err
	})
}

func buildCreateFunc(called *bool) cgo_testing.ReactionFunc {
	return cgo_testing.ReactionFunc(func(action cgo_testing.Action) (bool, runtime.Object, error) {
		*called = true
		return true, nil, nil
	})
}

func TestUpdateOrCreate(t *testing.T) {
	testCases := []struct {
		updateError   error
		createCalled  bool
		returnedError error
		description   string
	}{
		{normal, false, normal, "Failure to update a ConfigMap b/c of a generic error"},
		{k8sNotFound, true, nil, "Failure to update a ConfigMap b/c it does not exist"},
		{nil, false, nil, "ConfigMap updated successfully"},
	}

	for _, testCase := range testCases {
		fakeClient := &fake.FakeCoreV1{Fake: &cgo_testing.Fake{}}
		fakeClient.AddReactor("update", resource, buildUpdateFunc(testCase.updateError))
		createCalled := false
		fakeClient.AddReactor("create", resource, buildCreateFunc(&createCalled))

		sut := configMapImpl{
			client: fakeClient,
		}
		err := sut.UpdateOrCreate(namespace, &api.ConfigMap{})

		if err != testCase.returnedError {
			t.Errorf("UpdateOrCreate returned error %#v, want %#v", err, testCase.returnedError)
		}

		if createCalled != testCase.createCalled {
			t.Errorf("Create called is %t, want %t", createCalled, testCase.createCalled)
		}
	}
}
