#!/bin/bash
#
# Copyright 2020 Google LLC
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
set -x

SCRIPT_ROOT=$(readlink -f $(dirname ${BASH_SOURCE})/..)

ARTIFACTS=${ARTIFACTS:-"/tmp/artifacts"}
CLOUD_CONFIG=${CLOUD_CONFIG:-`gcloud info --format="value(config.paths.global_config_dir)"`}
CLOUD_SDK_ROOT=${CLOUD_SDK_ROOT:-`gcloud info --format="value(installation.sdk_root)"`}
DNS_ZONE=${DNS_ZONE:-"gkemanagedcerts-dev"}
DOMAIN=${DOMAIN:-"dev.gkemanagedcerts.certsbridge.com"}
KUBECONFIG=${KUBECONFIG:-"${HOME}/.kube/config"}
KUBERNETES_PROVIDER=${KUBERNETES_PROVIDER:-"gke"}
PLATFORM=${PLATFORM:-"gcp"}
PROJECT=${PROJECT:-`gcloud config list --format="value(core.project)"`}
PULL_NUMBER=${PULL_NUMBER:-""}
REGISTRY=${REGISTRY:-"gcr.io/gke-managed-certs"}
TAG=${TAG:-"ci_latest"}

while getopts "p:r:t:z:" opt; do
  case $opt in
    p) PLATFORM=$OPTARG ;;
    r) REGISTRY=$OPTARG ;;
    t) TAG=$OPTARG ;;
    z) DNS_ZONE=$OPTARG ;;
    :)
      echo "Option $OPTARG requires an argument." >&2
      exit 1
      ;;
  esac
done

if [ ! -z "$PULL_NUMBER" ]
then
  TAG="pr_${PULL_NUMBER}"
fi

mkdir /tmp/gcp_service_account
if [ $PLATFORM = "gcp" ]
then
  gsutil cp gs://${PROJECT}/key.json /tmp/gcp_service_account/key.json
fi

name=managed-certificate-controller
runner_image=${name}-runner
runner_path=/gopath/src/github.com/GoogleCloudPlatform/gke-managed-certs/

until docker build -t ${runner_image} ${SCRIPT_ROOT}/runner; do \
  echo "Building ${runner_image} image failed, retrying in 10 seconds..." && sleep 10; \
done

test -f /etc/service-account/service-account.json && \
  gcloud auth activate-service-account --key-file=/etc/service-account/service-account.json && \
  gcloud auth configure-docker || true

docker run -v ${SCRIPT_ROOT}:${runner_path} \
  -v ${CLOUD_SDK_ROOT}:${CLOUD_SDK_ROOT} \
  -v ${CLOUD_CONFIG}:/root/.config/gcloud \
  -v ${CLOUD_CONFIG}:/root/.config/gcloud-staging \
  -v ${KUBECONFIG}:/root/.kube/config \
  -v ${ARTIFACTS}:/tmp/artifacts \
  -v /tmp/gcp_service_account:/tmp/gcp_service_account \
  ${runner_image}:latest bash -c \
  "set -ex && cd ${runner_path} && dest=/tmp/artifacts; \
  rm -rf \${dest}/* && mkdir -p \${dest} && \
  { \
    CLOUD_SDK_ROOT=${CLOUD_SDK_ROOT} \
    GCP_SERVICE_ACCOUNT_FILE=/tmp/gcp_service_account/key.json \
    KUBECONFIG=\${HOME}/.kube/config \
    KUBERNETES_PROVIDER=${KUBERNETES_PROVIDER} \
    PROJECT=${PROJECT} \
    DNS_ZONE=${DNS_ZONE} \
    DOMAIN=${DOMAIN} \
    PLATFORM=${PLATFORM} \
    TAG=${TAG} \
    REGISTRY=${REGISTRY} \
    go test ./e2e/... -test.timeout=60m -logtostderr=false -alsologtostderr=true -v -log_dir=\${dest} \
      > \${dest}/e2e.out.txt && exitcode=\${?} || exitcode=\${?} ; \
  } && cat \${dest}/e2e.out.txt | go-junit-report > \${dest}/junit_01.xml && exit \${exitcode}"

rm -r /tmp/gcp_service_account
