all: build-dev

TAG?=dev
REGISTRY=eu.gcr.io
NAME=certs-controller
DOCKER_IMAGE=${REGISTRY}/managed-certs-gke/${NAME}:${TAG}

# Builds the managed certs controller binary
build-binary: clean deps
	godep go build -o ${NAME}

build-dev: clean deps
	godep go build ./...
	godep go build -o ${NAME}

# Builds the managed certs controller binary using a docker builder image
build-binary-in-docker: clean docker-builder
	docker run -v `pwd`:/gopath/src/managed-certs-gke/ ${NAME}-builder:latest bash -c 'cd /gopath/src/managed-certs-gke && make build-binary'

clean:
	rm -f ${NAME}

deps:
	go get github.com/tools/godep

# Builds and pushes a docker image with managed certs controller binary
docker:
	docker build --pull -t ${DOCKER_IMAGE} .
	docker push ${DOCKER_IMAGE}

# Builds a builder image, i. e. an image used to later build a managed certs binary.
docker-builder:
	docker build -t ${NAME}-builder builder

# Builds the managed certs controller binary, then a docker image with this binary, and pushes the image
release: build-binary-in-docker docker

.PHONY: all build-binary build-binary-in-docker build-dev clean deps docker docker-builder release
