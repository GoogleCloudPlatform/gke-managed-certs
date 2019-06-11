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

FROM gcr.io/distroless/base
LABEL maintainer="Krzysztof Kwaśniewski <krzyk@google.com>"

ADD managed-certificate-controller /managed-certificate-controller

ENTRYPOINT [ \
	"./managed-certificate-controller", \
	"--logtostderr=false", \
	"--alsologtostderr", \
	"-v=3", \
	"-log_file=/var/log/managed_certificate_controller.log" \
]
