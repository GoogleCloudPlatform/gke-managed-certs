#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(dirname ${BASH_SOURCE})/..

kubectl create namespace default
kubectl create -f ${SCRIPT_ROOT}/deploy/managedcertificates-crd.yaml --namespace=default
kubectl create -f ${SCRIPT_ROOT}/deploy/certs-controller.yaml --namespace=default
