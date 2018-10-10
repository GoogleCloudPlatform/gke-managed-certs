all: gofmt build-binary-in-docker run-test-in-docker clean cross

TAG?=dev
REGISTRY?=eu.gcr.io/managed-certs-gke
NAME=managed-certificate-controller
DOCKER_IMAGE=${REGISTRY}/${NAME}:${TAG}
RUNNER_IMAGE=${NAME}-runner
RUNNER_PATH=/gopath/src/github.com/GoogleCloudPlatform/gke-managed-certs/
KUBECONFIG?=${HOME}/.kube/config
KUBERNETES_PROVIDER?=gke

# Builds the managed certs controller binary
build-binary: clean deps
	godep go build -o ${NAME}

# Builds the managed certs controller binary using a docker runner image
build-binary-in-docker: docker-runner-builder
	docker run -v `pwd`:${RUNNER_PATH} ${RUNNER_IMAGE}:latest bash -c 'cd ${RUNNER_PATH} && make build-binary'

clean:
	rm -f ${NAME}

# Checks if Google criteria for releasing code as OSS are met
cross:
	/google/data/ro/teams/opensource/cross .

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

# Builds a runner image, i. e. an image used to build a managed-certificate-controller binary and to run its tests.
docker-runner-builder:
	docker build -t ${RUNNER_IMAGE} runner

e2e:
	KUBECONFIG=${KUBECONFIG} \
	KUBERNETES_PROVIDER=${KUBERNETES_PROVIDER} \
	godep go test ./e2e/... -v -test.timeout=60m

# Formats go source code with gofmt
gofmt:
	gofmt -w main.go
	find . -mindepth 1 -maxdepth 1 -name Godeps -o -name vendor -prune -o -type d -print | xargs gofmt -w

# Builds the managed certs controller binary, then a docker image with this binary, and pushes the image, for dev
release: build-binary-in-docker run-test-in-docker docker clean
	make -C http-hello

# Builds the managed certs controller binary, then a docker image with this binary, and pushes the image, for continuous integration
release-ci: build-binary-in-docker run-test-in-docker docker-ci
	make -C http-hello

run-e2e-in-docker: docker-runner-builder
	docker run -v `pwd`:${RUNNER_PATH} -v ${KUBECONFIG}:/root/.kube/config ${RUNNER_IMAGE}:latest bash -c 'cd ${RUNNER_PATH} && make e2e'

run-test-in-docker: docker-runner-builder
	docker run -v `pwd`:${RUNNER_PATH} ${RUNNER_IMAGE}:latest bash -c 'cd ${RUNNER_PATH} && make test'

test:
	godep go test ./pkg/... -cover

.PHONY: all build-binary build-binary-in-docker build-dev clean cross deps docker docker-runner-builder docker-ci e2e release release-ci run-e2e-in-docker run-test-in-docker test
