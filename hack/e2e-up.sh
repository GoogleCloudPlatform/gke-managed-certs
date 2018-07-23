#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(dirname ${BASH_SOURCE})/..

echo -ne "### Deploy CRD\n"
kubectl create -f ${SCRIPT_ROOT}/deploy/managedcertificates-crd.yaml

echo -ne "### Deploy ManagedCertificatesController\n"
kubectl create -f ${SCRIPT_ROOT}/deploy/certs-controller.yaml

echo -ne "### Deploy test1-certificate ManagedCertificate custom object\n"
kubectl create -f ${SCRIPT_ROOT}/deploy/test1-certificate.yaml

echo -ne "### Deploy ingress\n"
kubectl create -f ${SCRIPT_ROOT}/deploy/ingress.yaml
