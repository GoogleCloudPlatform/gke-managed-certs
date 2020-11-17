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

package e2e

import (
	"context"
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/api/errors"
)

func TestCRDValidation(t *testing.T) {
	ctx := context.Background()

	var domains101 []string
	for i := 0; i < 101; i++ {
		domains101 = append(domains101, fmt.Sprintf("too-many-%d.validation.example.com", i))
	}

	for i, tc := range []struct {
		desc    string
		domains []string
		success bool
	}{
		{
			"Domain with trailing dot not allowed",
			[]string{"trailing-dot.example.com."},
			false,
		},
		{
			"Domain with uppercase characters not allowed",
			[]string{"UPPER-CASE.example.com"},
			false,
		},
		{
			"Domain >63 characters not allowed",
			[]string{"very-long-domain-name-which-exceeds-the-limit-of-63-characters.validation.example.com"},
			false,
		},
		{
			"Domain with a wildcard not allowed",
			[]string{"*.validation.example.com"},
			false,
		},
		{
			"More than 100 SANs not allowed",
			domains101,
			false,
		},
		{
			"Single non-wildcard domain <=63 characters allowed",
			[]string{"validation.example.com"},
			true,
		},
		{
			"Multiple domain names allowed",
			[]string{"validation1.example.com", "validation2.example.com"},
			true,
		},
	} {
		i, tc := i, tc
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()
			name := fmt.Sprintf("crd-validation-%d", i)
			err := clients.ManagedCertificate.Create(ctx, name, tc.domains)
			if err == nil && !tc.success {
				t.Fatalf("Created, want failure")
			}
			defer clients.ManagedCertificate.Delete(ctx, name)

			if err == nil {
				return
			}

			statusErr, ok := err.(*errors.StatusError)
			if !ok {
				t.Fatalf("Creation failed with error %T, want errors.StatusError. Error: %s", err, err.Error())
			}

			if statusErr.Status().Reason != "Invalid" {
				t.Fatalf("Creation failed with reason %s, want Invalid, Error: %#v", statusErr.Status().Reason, err)
			}
		})
	}
}
