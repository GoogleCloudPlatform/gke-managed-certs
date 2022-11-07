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

package patch

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/api/networking/v1"

	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/testhelper/ingress"
	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/types"
)

func TestCreateMergePatch(t *testing.T) {
	t.Parallel()

	for description, testCase := range map[string]struct {
		originalIngress *v1.Ingress
		modifiedIngress *v1.Ingress
		wantModified    bool
		wantDiff        []byte
		wantErr         error
	}{
		"changes": {
			originalIngress: ingress.New(types.NewId("default", "foo"),
				ingress.AnnotationManagedCertificates("regular1,regular2"),
				ingress.AnnotationPreSharedCert("regular3")),
			modifiedIngress: ingress.New(types.NewId("default", "foo"),
				ingress.AnnotationManagedCertificates("regular1,regular2,regular3,regular4"),
				ingress.AnnotationPreSharedCert("regular1,regular4")),
			wantModified: true,
			wantDiff:     []byte("{\"metadata\":{\"annotations\":{\"ingress.gcp.kubernetes.io/pre-shared-cert\":\"regular1,regular4\",\"networking.gke.io/managed-certificates\":\"regular1,regular2,regular3,regular4\"}}}"),
		},
		"no changes": {
			originalIngress: ingress.New(types.NewId("default", "foo"),
				ingress.AnnotationManagedCertificates("regular1,regular2,deleted1,deleted2"),
				ingress.AnnotationPreSharedCert("regular1,regular2")),
			modifiedIngress: ingress.New(types.NewId("default", "foo"),
				ingress.AnnotationManagedCertificates("regular1,regular2,deleted1,deleted2"),
				ingress.AnnotationPreSharedCert("regular1,regular2")),
			wantModified: false,
			wantDiff:     nil,
		},
	} {
		tc := testCase
		t.Run(description, func(t *testing.T) {
			t.Parallel()

			diff, modified, err := CreateMergePatch(tc.originalIngress, tc.modifiedIngress)

			if err != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("CreateMergePatch() err: %v, want %v", err, testCase.wantErr)
			}
			if modified != tc.wantModified {
				t.Fatalf("CreateMergePatch() modified: %t, want: %t", modified, tc.wantModified)
			}
			if diff := cmp.Diff(diff, tc.wantDiff); diff != "" {
				t.Fatalf("CreateMergePatch() diff (-want, +got): %s", diff)
			}
		})
	}
}
