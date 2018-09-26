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
	"testing"

	"google.golang.org/api/googleapi"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var normal = errors.New("normal error")
var googleNormal = &googleapi.Error{
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
var k8sNormal = &k8s_errors.StatusError{
	metav1.Status{
		Code: 500,
	},
}
var k8sNotFound = &k8s_errors.StatusError{
	metav1.Status{
		Code: 404,
	},
}

var isNotFoundTestCases = []struct {
	in  error
	out bool
}{
	{nil, false},
	{normal, false},
	{googleNormal, false},
	{googleNotFound, true},
	{googleQuotaExceeded, false},
	{k8sNormal, false},
	{k8sNotFound, true},
}

func TestIsNotFound(t *testing.T) {
	for _, testCase := range isNotFoundTestCases {
		out := IsNotFound(testCase.in)
		if out != testCase.out {
			t.Errorf("IsNotFound(%v) = %t, want %t", testCase.in, out, testCase.out)
		}
	}
}

var isQuotaExceededTestCases = []struct {
	in  error
	out bool
}{
	{nil, false},
	{normal, false},
	{googleNormal, false},
	{googleNotFound, false},
	{googleQuotaExceeded, true},
	{k8sNormal, false},
	{k8sNotFound, false},
}

func TestIsQuotaExceeded(t *testing.T) {
	for _, testCase := range isQuotaExceededTestCases {
		out := IsQuotaExceeded(testCase.in)
		if out != testCase.out {
			t.Errorf("IsQuotaExceeded(%v) = %t, want %t", testCase.in, out, testCase.out)
		}
	}
}

var ignoreTestCases = []struct {
	in  error
	out error
}{
	{nil, nil},
	{normal, normal},
	{googleNormal, googleNormal},
	{googleNotFound, nil},
	{googleQuotaExceeded, googleQuotaExceeded},
	{k8sNormal, k8sNormal},
	{k8sNotFound, nil},
}

func TestIgnoreNotFound(t *testing.T) {
	for _, testCase := range ignoreTestCases {
		out := IgnoreNotFound(testCase.in)
		if out != testCase.out {
			t.Errorf("IgnoreNotFound(%v) = %v, want %v", testCase.in, out, testCase.out)
		}
	}
}
