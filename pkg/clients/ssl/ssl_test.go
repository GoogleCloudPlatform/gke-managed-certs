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

package ssl

import (
	"testing"

	computev1 "google.golang.org/api/compute/v1"
)

func TestError_IsQuotaExceeded(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		in  *Error
		out bool
	}{
		{
			&Error{&computev1.Operation{Error: &computev1.OperationError{}}},
			false,
		},
		{
			newFakeError("foo"),
			false,
		},
		{
			NewFakeQuotaExceededError(),
			true,
		},
	}

	for _, testCase := range testCases {
		out := testCase.in.IsQuotaExceeded()
		if out != testCase.out {
			t.Errorf("Error.IsQuotaExceeded(%#v) = %t, want %t", testCase.in, out, testCase.out)
		}
	}
}
