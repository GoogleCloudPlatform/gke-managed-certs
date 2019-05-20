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

# Display executed shell commands
set -x

SCRIPT_ROOT=$(dirname ${BASH_SOURCE})/..

PULL_NUMBER=${PULL_NUMBER:-""}
TAG=${TAG:-"ci_latest"}

if [ ! -z "$PULL_NUMBER" ]
then
  TAG="pr_${PULL_NUMBER}"
fi

make -C ${SCRIPT_ROOT} release-ci TAG=$TAG
