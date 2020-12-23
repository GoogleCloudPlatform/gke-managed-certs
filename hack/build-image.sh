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

PULL_NUMBER=${PULL_NUMBER:-""}
TAG=${TAG:-"ci_latest"}

if [ ! -z "$PULL_NUMBER" ]
then
  TAG="pr_${PULL_NUMBER}"
fi

REGISTRY=${REGISTRY:-"gcr.io/gke-managed-certs"}

# Latest commit hash for current branch
GIT_COMMIT=${GIT_COMMIT:-`git rev-parse HEAD`}
# This version-strategy uses git tags to set the version string
VERSION=${VERSION:-`git describe --tags --always --dirty`}

name=managed-certificate-controller
runner_image=${name}-runner
runner_path=/gopath/src/github.com/GoogleCloudPlatform/gke-managed-certs/

until docker build -t ${runner_image} ${SCRIPT_ROOT}/runner; do \
  echo "Building ${runner_image} image failed, retrying in 10 seconds..." && sleep 10; \
done

docker run -v ${SCRIPT_ROOT}:${runner_path} ${runner_image}:latest bash -c \
  "set -ex && cd ${runner_path} && \
  test -z \"\$(gofmt -l \$(find . -type f -name '*.go' | grep -v '/vendor/'))\" && \
  go vet ./... && \
  pkg=github.com/GoogleCloudPlatform/gke-managed-certs; \
  ld_flags=\"-X \${pkg}/pkg/version.Version=${VERSION} -X \${pkg}/pkg/version.GitCommit=${GIT_COMMIT}\"; \
  go build -o ${name} -ldflags \"\${ld_flags}\" && \
  go test ./pkg/... -cover"

test -f /etc/service-account/service-account.json && \
  gcloud auth activate-service-account --key-file=/etc/service-account/service-account.json && \
  gcloud auth configure-docker || true

until docker build --pull -t ${REGISTRY}/${name}:${TAG} -t ${REGISTRY}/${name}:${VERSION} ${SCRIPT_ROOT}; do \
  echo "Building ${name} image failed, retrying in 10 seconds..." && sleep 10; \
done
until docker push ${REGISTRY}/${name}:${TAG}; do \
  echo "Pushing ${name} image failed, retrying in 10 seconds..." && sleep 10; \
done
until docker push ${REGISTRY}/${name}:${VERSION}; do \
  echo "Pushing ${name} image failed, retrying in 10 seconds..." && sleep 10; \
done

rm -f ${SCRIPT_ROOT}/${name}
