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

export PROJECT=`gcloud config get-value project`
export NODE_SA_NAME=managed-certificate-controller
export NODE_SA_EMAIL=${NODE_SA_NAME}@${PROJECT}.iam.gserviceaccount.com

gcloud iam service-accounts create $NODE_SA_NAME --display-name "GKE Managed Certs controller"
until gcloud iam service-accounts describe $NODE_SA_EMAIL; do \
  echo "Fetching service account ${NODE_SA_EMAIL} failed, retrying in 10 seconds..." && sleep 10; \
done

gcloud projects add-iam-policy-binding gke-managed-certs --member serviceAccount:$NODE_SA_EMAIL --role roles/storage.objectViewer

gcloud projects add-iam-policy-binding $PROJECT --member serviceAccount:$NODE_SA_EMAIL --role roles/storage.objectViewer
gcloud projects add-iam-policy-binding $PROJECT --member serviceAccount:$NODE_SA_EMAIL --role roles/monitoring.metricWriter
gcloud projects add-iam-policy-binding $PROJECT --member serviceAccount:$NODE_SA_EMAIL --role roles/monitoring.viewer
gcloud projects add-iam-policy-binding $PROJECT --member serviceAccount:$NODE_SA_EMAIL --role roles/logging.logWriter
gcloud projects add-iam-policy-binding $PROJECT --member serviceAccount:$NODE_SA_EMAIL --role roles/compute.securityAdmin
