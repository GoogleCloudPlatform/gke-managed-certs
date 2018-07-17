#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(dirname ${BASH_SOURCE})/..

echo -ne "Deploy components for e2e tests"
${SCRIPT_ROOT}/hack/deploy-for-e2e.sh

echo -ne `(kubectl get pods -o yaml)`
