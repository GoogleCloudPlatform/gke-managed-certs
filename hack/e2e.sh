#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(dirname ${BASH_SOURCE})/..

echo -ne "Configure registry authentication"
gcloud auth activate-service-account --key-file=/etc/service-account/service-account.json
gcloud auth configure-docker

#echo -ne "Define context for kubectl\n"
#kubectl config set-context continuous_integration --namespace="continuous_integration"
#kubectl config use-context continuous_integration

echo -ne "Delete components created for e2e tests\n"
${SCRIPT_ROOT}/hack/delete-for-e2e.sh

echo -ne "Deploy components for e2e tests\n"
${SCRIPT_ROOT}/hack/deploy-for-e2e.sh

echo -ne `(kubectl get pods -o yaml)`

echo -ne "Delete components created for e2e tests\n"
${SCRIPT_ROOT}/hack/delete-for-e2e.sh
