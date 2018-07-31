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

echo -ne "### sleep 60 sec\n"
sleep 60

echo -ne "### expect 2 SslCertificate resources..."
sslCertificates=($(gcloud alpha compute ssl-certificates list --uri))

if [ ${#sslCertificates[@]} -ne 2 ];
then
  echo -ne "instead found the following: ${sslCertificates:-}\n"
  exit 1
else
  echo -ne "ok\n"
fi

echo -ne "### remove annotation managed-certificates from ingress\n"
kubectl annotate ingress test-ingress cloud.google.com/managed-certificates-

echo -ne "### remove custom resources test1-certificate and test2-certificate\n"
kubectl delete -f ${SCRIPT_ROOT}/deploy/test1-certificate.yaml
kubectl delete -f ${SCRIPT_ROOT}/deploy/test2-certificate.yaml

echo -ne "### sleep 60 sec\n"
sleep 60

echo -ne "### expect 0 SslCertificate resources..."
sslCertificates=($(gcloud alpha compute ssl-certificates list --uri))

if [ ${#sslCertificates[@]} -ne 0 ];
then
  echo -ne "instead found the following: ${sslCertificates:-}\n"
  exit 1
else
  echo -ne "ok\n"
fi

exit 0
