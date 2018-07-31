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
