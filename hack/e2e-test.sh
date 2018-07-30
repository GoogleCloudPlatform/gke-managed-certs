#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

echo -ne "### `kubectl get ingress`\n"

echo -ne "### sleep 60 sec\n"
sleep 60

echo -ne "### expect 2 SslCertificate resources..."
sslCertificates=($(gcloud alpha compute ssl-certificates  list --uri))

if [ ${#sslCertificates[@]} -ne 2 ];
then
  echo -ne "instead found the following: ${sslCertificates}\n"
  exit 1
else
  echo -ne "ok\n"
fi

exit 0
