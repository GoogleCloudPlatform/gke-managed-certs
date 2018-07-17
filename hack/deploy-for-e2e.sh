#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(dirname ${BASH_SOURCE})/..

echo -ne "Configure registry authentication"
gcloud auth activate-service-account --key-file=/etc/service-account/service-account.json
gcloud auth configure-docker

kubectl create -f ${SCRIPT_ROOT}/deploy/managedcertificates-crd.yaml
kubectl create -f ${SCRIPT_ROOT}/deploy/certs-controller.yaml
