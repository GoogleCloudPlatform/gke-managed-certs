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

export NODE_SA_NAME=mcrt-controller-sa
gcloud iam service-accounts create $NODE_SA_NAME --display-name "managed-certificate-controller service account"
export NODE_SA_EMAIL=`gcloud iam service-accounts list --format='value(email)' --filter='displayName:managed-certificate-controller'`

export PROJECT=`gcloud config get-value project`
gcloud projects add-iam-policy-binding $PROJECT --member serviceAccount:$NODE_SA_EMAIL --role roles/monitoring.metricWriter
gcloud projects add-iam-policy-binding $PROJECT --member serviceAccount:$NODE_SA_EMAIL --role roles/monitoring.viewer
gcloud projects add-iam-policy-binding $PROJECT --member serviceAccount:$NODE_SA_EMAIL --role roles/logging.logWriter
gcloud projects add-iam-policy-binding $PROJECT --member serviceAccount:$NODE_SA_EMAIL --role roles/compute.loadBalancerAdmin

gcloud iam service-accounts keys create ./key.json --iam-account $NODE_SA_EMAIL

gsutil mb -b on gs://${PROJECT}
gsutil cp key.json gs://${PROJECT}/key.json
rm key.json
