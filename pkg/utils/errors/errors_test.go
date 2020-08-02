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

package errors

import (
	"errors"
	"fmt"
	"testing"

	"google.golang.org/api/googleapi"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var errGeneric = errors.New("generic error")
var errCompute404 = &googleapi.Error{Code: 404}
var errCompute500 = &googleapi.Error{Code: 500}
var errK8sInternal = k8serrors.NewInternalError(fmt.Errorf("test_internal_error"))
var errK8sNotFound = k8serrors.NewNotFound(schema.GroupResource{
	Group:    "test_group",
	Resource: "test_resource",
}, "test_name")

func TestIsNotFound(t *testing.T) {
	testCases := []struct {
		in  error
		out bool
	}{
		{nil, false},
		{errGeneric, false},
		{NotFound, true},
		{errCompute404, true},
		{errCompute500, false},
		{errK8sInternal, false},
		{errK8sNotFound, true},
	}

	for _, testCase := range testCases {
		out := IsNotFound(testCase.in)
		if out != testCase.out {
			t.Errorf("IsNotFound(%#v) = %t, want %t", testCase.in, out, testCase.out)
		}
	}
}

func TestIgnoreNotFound(t *testing.T) {
	testCases := []struct {
		in  error
		out error
	}{
		{nil, nil},
		{errGeneric, errGeneric},
		{NotFound, nil},
		{errCompute404, nil},
		{errCompute500, errCompute500},
		{errK8sInternal, errK8sInternal},
		{errK8sNotFound, nil},
	}

	for _, testCase := range testCases {
		out := IgnoreNotFound(testCase.in)
		if out != testCase.out {
			t.Errorf("IgnoreNotFound(%#v) = %v, want %v", testCase.in, out, testCase.out)
		}
	}
}
