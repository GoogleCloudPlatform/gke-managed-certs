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

SCRIPT_ROOT=$(dirname ${BASH_SOURCE})/..

go get -d k8s.io/code-generator/...

# Checkout code-generator to compatible version
(cd $GOPATH/src/k8s.io/code-generator && git checkout 3dcf91f64f638563e5106f21f50c31fa361c918d)

REPOSITORY=github.com/GoogleCloudPlatform/gke-managed-certs
$GOPATH/src/k8s.io/code-generator/generate-groups.sh all \
  $REPOSITORY/pkg/clientgen $REPOSITORY/pkg/apis networking.gke.io:v1beta1 \
  --go-header-file $SCRIPT_ROOT/hack/header.go.txt

# This hack is required as the autogens don't work for upper case letters in package names.
# This issue: https://github.com/kubernetes/code-generator/issues/22 needs to be resolved to remove this hack.
REPOSITORY_LOWER=`echo "$REPOSITORY" | tr '[:upper:]' '[:lower:]'`
rm -rf $GOPATH/src/$REPOSITORY/pkg/clientgen/clientset/versioned/typed
mv $GOPATH/src/$REPOSITORY_LOWER/pkg/clientgen/clientset/versioned/typed $GOPATH/src/$REPOSITORY/pkg/clientgen/clientset/versioned
find $GOPATH/src/$REPOSITORY/pkg/clientgen -name "*.go" | xargs sed -i 's/googlecloudplatform/GoogleCloudPlatform/g'
