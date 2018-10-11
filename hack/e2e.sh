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
GET_SSL_CERTIFICATES="gcloud compute ssl-certificates list --uri"
DNS_ZONE="managedcertsgke"
DNS_PROJECT="certsbridge-dev"

# Calls either kubectl create or kubectl delete on all k8s yaml files in the
# deploy/ directory, depending on argument
function kubectl_all {
  for file in `ls ${SCRIPT_ROOT}/deploy`
  do
    echo "kubectl $1 $file"
    ignore="--ignore-not-found=true"
    if [ $1 == "create" ]
    then
      ignore=""
    fi

    kubectl $1 -f ${SCRIPT_ROOT}/deploy/$file $ignore
  done
}

function delete_managed_certificates {
  echo "Delete all ManagedCertificate objects"
  for name in `kubectl get mcrt -o go-template='{{range .items}}{{.metadata.name}}{{"\n"}}{{end}}'`
  do
    kubectl delete mcrt $name
  done
}

function delete_ssl_certificates {
  echo "Delete all SslCertificate objects"
  for uri in `$GET_SSL_CERTIFICATES`
  do
    echo y | gcloud compute ssl-certificates delete $uri || true
  done

  [[ `$GET_SSL_CERTIFICATES | wc --lines` == "0" ]]
}

function clear_dns {
  echo "Remove all A sub-records of com.certsbridge from a dns zone $DNS_ZONE"
  arg="--zone $DNS_ZONE --project $DNS_PROJECT"
  gcloud dns record-sets transaction start $arg

  for line in `gcloud dns record-sets list $arg --filter=type=A | grep certsbridge.com | tr -s ' ' | tr ' ' ';' | cut -d ';' -f 1,4`
  do
    lineArray=(${line//;/ })
    gcloud dns record-sets transaction remove $arg --name="${lineArray[0]}" --type=A --ttl=300 ${lineArray[1]}
  done

  gcloud dns record-sets transaction execute $arg
}

function backoff {
  timeout=30
  max_attempts=40

  for i in `seq $max_attempts`
  do
    eval $1 && result=$? || result=$?
    if [ $result == 0 ]
    then
      return 0
    fi

    echo "Condition not met, retry in $timeout seconds"
    sleep $timeout
  done

  return 1
}

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

    echo "Install godep"
    go get github.com/tools/godep
  fi
}

function tear_down {
  kubectl_all "delete"
  delete_managed_certificates
  backoff delete_ssl_certificates
  clear_dns
}

function set_up {
  kubectl_all "create"
}

function main {
  init
  tear_down
  set_up

  ${SCRIPT_ROOT}/hack/e2e.py --zone=$DNS_ZONE && exitcodepy=$? || exitcodepy=$?

  make -C ${SCRIPT_ROOT} run-e2e-in-docker && exitcode=$? || exitcode=$?

  tear_down

  exit $exitcodepy && $exitcode
}

while getopts "z:" opt; do
  case $opt in
    z)
      DNS_ZONE=$OPTARG
      ;;
    :)
      echo "Option $OPTARG requires an argument." >&2
      exit 1
      ;;
  esac
done

main
