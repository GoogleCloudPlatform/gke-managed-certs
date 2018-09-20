#!/bin/bash
#
# Copyright 2018 Google LLC
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

go get -d k8s.io/code-generator/...

REPOSITORY=github.com/GoogleCloudPlatform/gke-managed-certs
$GOPATH/src/k8s.io/code-generator/generate-groups.sh all \
  $REPOSITORY/third_party/client $REPOSITORY/pkg/apis gke.googleapis.com:v1alpha1

# This hack is required as the autogens don't work for upper case letters in package names.
# This issue: https://github.com/kubernetes/code-generator/issues/22 needs to be resolved to remove this hack.
REPOSITORY_LOWER=`echo "$REPOSITORY" | tr '[:upper:]' '[:lower:]'`
mv $GOPATH/src/$REPOSITORY_LOWER/third_party/client/clientset/versioned/typed $GOPATH/src/$REPOSITORY/third_party/client/clientset/versioned
find $GOPATH/src/$REPOSITORY/third_party -name "*.go" | xargs sed -i 's/googlecloudplatform/GoogleCloudPlatform/g'
