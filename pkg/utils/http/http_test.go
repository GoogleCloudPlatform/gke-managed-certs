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

package http

import (
	"errors"
	"fmt"
	"testing"

	"google.golang.org/api/googleapi"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var generic = errors.New("generic error")
var googleInternal = &googleapi.Error{
	Code: 500,
}
var googleNotFound = &googleapi.Error{
	Code: 404,
}
var googleQuotaExceeded = &googleapi.Error{
	Code: 403,
	Errors: []googleapi.ErrorItem{
		googleapi.ErrorItem{
			Reason: "quotaExceeded",
		},
	},
}
var k8sInternal = k8s_errors.NewInternalError(fmt.Errorf("test_internal_error"))
var k8sNotFound = k8s_errors.NewNotFound(schema.GroupResource{
	Group:    "test_group",
	Resource: "test_resource",
}, "test_name")

func TestIsNotFound(t *testing.T) {
	testCases := []struct {
		in  error
		out bool
	}{
		{nil, false},
		{generic, false},
		{googleInternal, false},
		{googleNotFound, true},
		{googleQuotaExceeded, false},
		{k8sInternal, false},
		{k8sNotFound, true},
	}

	for _, testCase := range testCases {
		out := IsNotFound(testCase.in)
		if out != testCase.out {
			t.Errorf("IsNotFound(%#v) = %t, want %t", testCase.in, out, testCase.out)
		}
	}
}

func TestIsQuotaExceeded(t *testing.T) {
	testCases := []struct {
		in  error
		out bool
	}{
		{nil, false},
		{generic, false},
		{googleInternal, false},
		{googleNotFound, false},
		{googleQuotaExceeded, true},
		{k8sInternal, false},
		{k8sNotFound, false},
	}

	for _, testCase := range testCases {
		out := IsQuotaExceeded(testCase.in)
		if out != testCase.out {
			t.Errorf("IsQuotaExceeded(%#v) = %t, want %t", testCase.in, out, testCase.out)
		}
	}
}

func TestIgnoreNotFound(t *testing.T) {
	testCases := []struct {
		in  error
		out error
	}{
		{nil, nil},
		{generic, generic},
		{googleInternal, googleInternal},
		{googleNotFound, nil},
		{googleQuotaExceeded, googleQuotaExceeded},
		{k8sInternal, k8sInternal},
		{k8sNotFound, nil},
	}

	for _, testCase := range testCases {
		out := IgnoreNotFound(testCase.in)
		if out != testCase.out {
			t.Errorf("IgnoreNotFound(%#v) = %v, want %v", testCase.in, out, testCase.out)
		}
	}
}
