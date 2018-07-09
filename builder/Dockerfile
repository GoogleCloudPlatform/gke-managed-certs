FROM golang:1.10.2

ENV GOPATH /gopath/
ENV PATH $GOPATH/bin:$PATH

RUN apt-get update && apt-get --yes install libseccomp-dev
RUN go version
RUN go get github.com/tools/godep
RUN godep version
CMD ["/bin/bash"]
