#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

go get -d k8s.io/code-generator/...

$GOPATH/src/k8s.io/code-generator/generate-groups.sh all \
  managed-certs-gke/pkg/client managed-certs-gke/pkg/apis cloud.google.com:v1alpha1
