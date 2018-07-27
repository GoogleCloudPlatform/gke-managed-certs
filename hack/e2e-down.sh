#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(dirname ${BASH_SOURCE})/..

echo -ne "### Remove all existing SslCertificate objects\n"
for uri in `gcloud alpha compute ssl-certificates list --uri`; do
  echo -ne "Y\n" || gcloud alpha compute ssl-certificates delete $uri
done

echo -ne "### Delete ManagedCertificatesController\n"
kubectl delete -f ${SCRIPT_ROOT}/deploy/certs-controller.yaml --ignore-not-found=true

echo -ne "### Delete CRD\n"
kubectl delete -f ${SCRIPT_ROOT}/deploy/managedcertificates-crd.yaml --ignore-not-found=true

echo -ne "### Delete ingress\n"
kubectl delete -f ${SCRIPT_ROOT}/deploy/ingress.yaml --ignore-not-found=true

echo -ne "### Remove RBAC\n"
kubectl delete -f ${SCRIPT_ROOT}/deploy/rbac.yaml --ignore-not-found=true
