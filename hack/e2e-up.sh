#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(dirname ${BASH_SOURCE})/..

kubectl create -f ${SCRIPT_ROOT}/deploy/managedcertificates-crd.yaml
kubectl create -f ${SCRIPT_ROOT}/deploy/certs-controller.yaml

kubectl create -f ${SCRIPT_ROOT}/deploy/test-certificate.yaml
