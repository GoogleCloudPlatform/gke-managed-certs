#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(dirname ${BASH_SOURCE})/..
echo -ne "### cd to hack\n"
cd ${SCRIPT_ROOT}/hack
echo -ne "### pwd: `pwd`\n"

echo -ne "### Configure registry authentication\n"
gcloud auth activate-service-account --key-file=/etc/service-account/service-account.json
gcloud auth configure-docker

echo -ne "### get kubectl 1.11\n"
curl -LO https://storage.googleapis.com/kubernetes-release/release/v1.11.0/bin/linux/amd64/kubectl
chmod +x kubectl
echo -ne "### kubectl version: `./kubectl version`\n"

echo -ne "### set namespace default\n"
kubectl config set-context $(kubectl config current-context) --namespace=default

echo -ne "### Delete components created for e2e tests\n"
./e2e-down.sh

echo -ne "### Deploy components for e2e tests\n"
./e2e-up.sh

###
# Invoke test code
###

echo -ne "### `kubectl get ingress`\n"

echo -ne "### sleep 60 sec\n"
sleep 60

echo -ne "### all logs\n"
for p in $(kubectl get pods -o go-template='{{range .items}}{{.metadata.name}}{{"\n"}}{{end}}'); do kubectl logs $p; done

echo -ne "### list ssl certificates\n"
gcloud alpha compute ssl-certificates list --uri

echo -ne "sleep 5 minutes\n"
sleep 300

echo -ne "### all logs\n"
for p in $(kubectl get pods -o go-template='{{range .items}}{{.metadata.name}}{{"\n"}}{{end}}'); do kubectl logs $p; done

echo -ne "### list ssl certificates\n"
gcloud alpha compute ssl-certificates list --uri

###
# End of test code
###

echo -ne "### Delete components created for e2e tests\n"
./e2e-down.sh
