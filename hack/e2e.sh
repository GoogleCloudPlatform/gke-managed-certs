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
SERVICE_ACCOUNT_KEY="/etc/service-account/service-account.json"

function init {
  if [ -f $SERVICE_ACCOUNT_KEY ]
  then
    echo "Configuring registry authentication"
    gcloud auth activate-service-account --key-file=${SERVICE_ACCOUNT_KEY}
    gcloud auth configure-docker

    echo "Install kubectl 1.11"
    curl -L -o kubectl https://storage.googleapis.com/kubernetes-release/release/v1.11.0/bin/linux/amd64/kubectl
    chmod +x kubectl
  fi

  export PATH=$PWD:$PATH
  echo "Prepend \$PATH with CWD: $PATH"
  echo "Kubectl version: `kubectl version`"

  if [ -f $SERVICE_ACCOUNT_KEY ]
  then
    echo "Set namespace default"
    kubectl config set-context `kubectl config current-context` --namespace=default
  fi
}

function main {
  init

  ${SCRIPT_ROOT}/hack/e2e.py $@ && exitcode=$? || exitcode=$?

  #go test ${SCRIPT_ROOT}/e2e/*go -v -test.timeout=60m --args --ginkgo.v=true --report-dir=/workspace/_artifacts --disable-log-dump

  exit $exitcode
}

main
