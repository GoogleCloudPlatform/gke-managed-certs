#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(dirname ${BASH_SOURCE})/..

echo -ne "### Deploy RBAC\n"
kubectl create -f ${SCRIPT_ROOT}/deploy/rbac.yaml

echo -ne "### Deploy CRD\n"
kubectl create -f ${SCRIPT_ROOT}/deploy/managedcertificates-crd.yaml

echo -ne "### Deploy ManagedCertificatesController\n"
kubectl create -f ${SCRIPT_ROOT}/deploy/managed-certificate-controller.yaml

echo -ne "### Deploy test1-certificate and test2-certificate ManagedCertificate custom objects\n"
kubectl create -f ${SCRIPT_ROOT}/deploy/test1-certificate.yaml
kubectl create -f ${SCRIPT_ROOT}/deploy/test2-certificate.yaml

echo -ne "### Deploy ingress\n"
kubectl create -f ${SCRIPT_ROOT}/deploy/ingress.yaml
