#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(dirname ${BASH_SOURCE})/..

kubectl delete -f ${SCRIPT_ROOT}/deploy/managedcertificates-crd.yaml --ignore-not-found=true --namespace=default
kubectl delete -f ${SCRIPT_ROOT}/deploy/certs-controller.yaml --ignore-not-found=true --namespace=default
