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

echo -ne "### Delete managed-certificate-controller\n"
kubectl delete -f ${SCRIPT_ROOT}/deploy/managed-certificate-controller.yaml --ignore-not-found=true

echo -ne "### Delete CRD\n"
kubectl delete -f ${SCRIPT_ROOT}/deploy/managedcertificates-crd.yaml --ignore-not-found=true

echo -ne "### Delete ingress\n"
kubectl delete -f ${SCRIPT_ROOT}/deploy/ingress.yaml --ignore-not-found=true

echo -ne "### Delete http-hello service\n"
kubectl delete -f ${SCRIPT_ROOT}/deploy/http-hello.yaml --ignore-not-found=true

echo -ne "### Remove RBAC\n"
kubectl delete -f ${SCRIPT_ROOT}/deploy/rbac.yaml --ignore-not-found=true

echo -ne "### Remove all existing SslCertificate objects\n"
SLEEP=1
for i in `seq 10`; do
  SSL_CERTIFICATES=($(gcloud alpha compute ssl-certificates list --uri))
  sleep $SLEEP

  if [ ${#SSL_CERTIFICATES[@]} -eq 0 ];
  then
    break
  fi

  for uri in ${SSL_CERTIFICATES}; do
    echo y | gcloud alpha compute ssl-certificates delete $uri || true
  done

  SLEEP=$(( $SLEEP * 2 ))
done
