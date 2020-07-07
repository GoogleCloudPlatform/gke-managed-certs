#!/bin/bash
#
# Copyright 2020 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(readlink -f $(dirname ${BASH_SOURCE})/..)

REPOSITORY=github.com/GoogleCloudPlatform/gke-managed-certs
bash ${SCRIPT_ROOT}/vendor/k8s.io/code-generator/generate-groups.sh all \
  $REPOSITORY/pkg/clientgen $REPOSITORY/pkg/apis networking.gke.io:v1beta1,v1beta2,v1 \
  --output-base ${SCRIPT_ROOT}/../../.. \
  --go-header-file ${SCRIPT_ROOT}/hack/header.go.txt
