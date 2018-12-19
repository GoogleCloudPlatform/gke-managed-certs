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

FROM golang:1.11
LABEL maintainer="Krzysztof Kwaśniewski <krzyk@google.com>"

ENV GOPATH /gopath/
ENV PATH $GOPATH/bin:$PATH

RUN apt-get update && apt-get --yes install libseccomp-dev && go version \
	&& go get github.com/tools/godep && godep version \
	&& go get github.com/jstemmer/go-junit-report
CMD ["/bin/bash"]
