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
SERVICE_ACCOUNT_KEY="/etc/service-account/service-account.json"

CRD_VALIDATION_ENABLED=${CRD_VALIDATION_ENABLED:-true}
DNS_ZONE=${DNS_ZONE:-"managedcertsgke"}
PLATFORM=${PLATFORM:-"GCP"}

function init {
  if [ -f $SERVICE_ACCOUNT_KEY ]
  then
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

function tear_down {
  kubectl delete -f ${SCRIPT_ROOT}/deploy/http-hello.yaml --ignore-not-found=true
  if [ $PLATFORM = "GCP" ]
  then
    kubectl delete -f ${SCRIPT_ROOT}/deploy/managedcertificates-crd.yaml --ignore-not-found=true
    kubectl delete -f ${SCRIPT_ROOT}/deploy/managed-certificate-controller.yaml --ignore-not-found=true
  fi
}

function set_up {
  kubectl create -f ${SCRIPT_ROOT}/deploy/http-hello.yaml

  if [ $PLATFORM = "GCP" ]
  then
    kubectl create -f ${SCRIPT_ROOT}/deploy/managedcertificates-crd.yaml
    kubectl create -f ${SCRIPT_ROOT}/deploy/managed-certificate-controller.yaml
  fi
}

function main {
  init
  tear_down
  set_up

  make -C ${SCRIPT_ROOT} run-e2e-in-docker \
    CRD_VALIDATION_ENABLED=$CRD_VALIDATION_ENABLED DNS_ZONE=$DNS_ZONE && exitcode=$? || exitcode=$?

  tear_down

  exit $exitcode
}

while getopts "p:v:z:" opt; do
  case $opt in
    p) PLATFORM=$OPTARG ;;
    v) CRD_VALIDATION_ENABLED=$OPTARG ;;
    z) DNS_ZONE=$OPTARG ;;
    :)
      echo "Option $OPTARG requires an argument." >&2
      exit 1
      ;;
  esac
done

main
