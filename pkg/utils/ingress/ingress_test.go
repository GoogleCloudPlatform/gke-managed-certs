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

package ingress

import (
	"testing"

	"k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/config"
)

func TestIsGKE(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		desc    string
		ingress *v1.Ingress
		wantGKE bool
	}{
		{
			desc:    "nil object",
			ingress: nil,
			wantGKE: false,
		},
		{
			desc:    "lack of annotations",
			ingress: &v1.Ingress{},
			wantGKE: true,
		},
		{
			desc:    "lack of Ingress class annotation",
			ingress: buildIngress("foo", "bar"),
			wantGKE: true,
		},
		{
			desc:    "empty Ingress class annotation",
			ingress: buildIngress(config.AnnotationIngressClassKey, ""),
			wantGKE: true,
		},
		{
			desc:    "Ingress class annotation equals gce",
			ingress: buildIngress(config.AnnotationIngressClassKey, "gce"),
			wantGKE: true,
		},
		{
			desc:    "Ingress class annotation unknown value",
			ingress: buildIngress(config.AnnotationIngressClassKey, "unknown"),
			wantGKE: false,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.desc, func(t *testing.T) {
			t.Parallel()

			got := IsGKE(testCase.ingress)
			if got != testCase.wantGKE {
				t.Errorf("IsGKE(%+v): %t, want %t", testCase.ingress, got, testCase.wantGKE)
			}
		})
	}
}

func buildIngress(key, value string) *v1.Ingress {
	return &v1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{key: value},
		},
	}
}
