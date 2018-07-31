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
echo -ne "### cd to hack\n"
cd ${SCRIPT_ROOT}/hack
echo -ne "### pwd: `pwd`\n"

echo -ne "### Configure registry authentication\n"
gcloud auth activate-service-account --key-file=/etc/service-account/service-account.json
gcloud auth configure-docker

echo -ne "### get kubectl 1.11\n"
curl -LO https://storage.googleapis.com/kubernetes-release/release/v1.11.0/bin/linux/amd64/kubectl
chmod +x kubectl
echo -ne "### kubectl version: `./kubectl version`\n"

echo -ne "### set namespace default\n"
kubectl config set-context $(kubectl config current-context) --namespace=default

echo -ne "### Delete components created for e2e tests\n"
./e2e-down.sh

echo -ne "### Deploy components for e2e tests\n"
./e2e-up.sh

###
# Invoke test code
###

./e2e-test.sh && exitcode=$? || exitcode=$?

###
# End of test code
###

echo -ne "### Delete components created for e2e tests\n"
./e2e-down.sh

exit $exitcode
