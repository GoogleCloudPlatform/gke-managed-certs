all: build-binary-in-docker

TAG?=dev
REGISTRY?=eu.gcr.io/managed-certs-gke
NAME=managed-certs-controller
DOCKER_IMAGE=${REGISTRY}/${NAME}:${TAG}

# Builds the managed certs controller binary
build-binary: clean deps
	godep go build -o ${NAME}

# Builds the managed certs controller binary using a docker builder image
build-binary-in-docker: clean docker-builder test
	docker run -v `pwd`:/gopath/src/managed-certs-gke/ ${NAME}-builder:latest bash -c 'cd /gopath/src/managed-certs-gke && make build-binary'

clean:
	rm -f ${NAME}

deps:
	go get github.com/tools/godep

# Builds and pushes a docker image with managed certs controller binary
docker:
	docker build --pull -t ${DOCKER_IMAGE} .
	docker push ${DOCKER_IMAGE}

# Builds and pushes a docker image with managed certs controller binary - for CI, activates a service account before pushing
docker-ci:
	docker build --pull -t ${DOCKER_IMAGE} .
	gcloud auth activate-service-account --key-file=/etc/service-account/service-account.json
	gcloud auth configure-docker
	docker push ${DOCKER_IMAGE}

# Builds a builder image, i. e. an image used to later build a managed certs binary.
docker-builder:
	docker build -t ${NAME}-builder builder

# Builds the managed certs controller binary, then a docker image with this binary, and pushes the image, for dev
release: build-binary-in-docker docker

# Builds the managed certs controller binary, then a docker image with this binary, and pushes the image, for continuous integration
release-ci: build-binary-in-docker docker-ci

test:
	find pkg -name '*_test.go' -exec dirname '{}' \; | sed -e 's/\(.*\)/.\/\1/g' | xargs godep go test

.PHONY: all build-binary build-binary-in-docker build-dev clean deps docker docker-builder docker-ci release release-ci test
