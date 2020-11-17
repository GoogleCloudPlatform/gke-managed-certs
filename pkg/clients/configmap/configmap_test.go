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

package configmap

import (
	"context"
	"errors"
	"testing"

	api "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	fakev1 "k8s.io/client-go/kubernetes/typed/core/v1/fake"
	cgotesting "k8s.io/client-go/testing"
)

const (
	namespace = "default"
	resource  = "configmaps"
)

var generic = errors.New("generic error")
var k8sNotFound = k8serrors.NewNotFound(schema.GroupResource{
	Group:    "test_group",
	Resource: "test_resource",
}, "test_name")

func buildUpdateFunc(err error) cgotesting.ReactionFunc {
	return cgotesting.ReactionFunc(func(action cgotesting.Action) (bool, runtime.Object, error) {
		return true, nil, err
	})
}

func buildCreateFunc(called *bool) cgotesting.ReactionFunc {
	return cgotesting.ReactionFunc(func(action cgotesting.Action) (bool, runtime.Object, error) {
		*called = true
		return true, nil, nil
	})
}

func TestUpdateOrCreate(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		updateError   error
		createCalled  bool
		returnedError error
		description   string
	}{
		{generic, false, generic, "Failure to update a ConfigMap b/c of a generic error"},
		{k8sNotFound, true, nil, "Failure to update a ConfigMap b/c it does not exist"},
		{nil, false, nil, "ConfigMap updated successfully"},
	}

	for _, testCase := range testCases {
		fakeClient := &fakev1.FakeCoreV1{Fake: &cgotesting.Fake{}}
		fakeClient.AddReactor("update", resource, buildUpdateFunc(testCase.updateError))
		createCalled := false
		fakeClient.AddReactor("create", resource, buildCreateFunc(&createCalled))

		configMap := impl{
			client: fakeClient,
		}
		err := configMap.UpdateOrCreate(ctx, namespace, &api.ConfigMap{})

		if err != testCase.returnedError {
			t.Errorf("UpdateOrCreate returned error %#v, want %#v", err, testCase.returnedError)
		}

		if createCalled != testCase.createCalled {
			t.Errorf("Create called is %t, want %t", createCalled, testCase.createCalled)
		}
	}
}
