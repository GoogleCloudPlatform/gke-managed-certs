all: gofmt build-binary-in-docker run-test-in-docker clean cross

TAG ?= dev
REGISTRY ?= gcr.io/k8s-image-staging
KUBECONFIG ?= ${HOME}/.kube/config
KUBERNETES_PROVIDER ?= gke
ARTIFACTS ?= /tmp/artifacts
CLOUD_CONFIG ?= $(shell gcloud info --format="value(config.paths.global_config_dir)")
CLOUD_SDK_ROOT ?= $(shell gcloud info --format="value(installation.sdk_root)")

# Latest commit hash for current branch
GIT_COMMIT ?= $(shell git rev-parse HEAD)
# This version-strategy uses git tags to set the version string
VERSION ?= $(shell git describe --tags --always --dirty)

name=managed-certificate-controller
runner_image=${name}-runner
runner_path=/gopath/src/github.com/GoogleCloudPlatform/gke-managed-certs/

auth-configure-docker:
	test -f /etc/service-account/service-account.json && \
		gcloud auth activate-service-account --key-file=/etc/service-account/service-account.json && \
		gcloud auth configure-docker || true

# Builds the managed certs controller binary
build-binary: clean deps
	pkg=github.com/GoogleCloudPlatform/gke-managed-certs; \
	ld_flags="-X $${pkg}/pkg/version.Version=${VERSION} -X $${pkg}/pkg/version.GitCommit=${GIT_COMMIT}"; \
	godep go build -o ${name} -ldflags "$${ld_flags}"

# Builds the managed certs controller binary using a docker runner image
build-binary-in-docker: docker-runner-builder
	docker run -v `pwd`:${runner_path} ${runner_image}:latest bash -c 'cd ${runner_path} && make build-binary GIT_COMMIT=${GIT_COMMIT} VERSION=${VERSION}'

clean:
	rm -f ${name}

# Checks if Google criteria for releasing code as OSS are met
cross:
	/google/data/ro/teams/opensource/cross .

deps:
	go get github.com/tools/godep

# Builds and pushes a docker image with managed certs controller binary
docker: auth-configure-docker
	docker build --pull -t ${REGISTRY}/${name}:${TAG} -t ${REGISTRY}/${name}:${VERSION} .
	docker push ${REGISTRY}/${name}:${TAG}

# Builds a runner image, i. e. an image used to build a managed-certificate-controller binary and to run its tests.
docker-runner-builder:
	docker build -t ${runner_image} runner

e2e:
	mkdir -p /tmp/artifacts && \
	CLOUD_SDK_ROOT=${CLOUD_SDK_ROOT} \
	KUBECONFIG=${KUBECONFIG} \
	KUBERNETES_PROVIDER=${KUBERNETES_PROVIDER} \
	godep go test ./e2e/... -v -test.timeout=60m | go-junit-report > /tmp/artifacts/junit_01.xml

# Formats go source code with gofmt
gofmt:
	gofmt -w main.go
	find . -mindepth 1 -maxdepth 1 -name Godeps -o -name vendor -prune -o -type d -print | xargs gofmt -w

# Builds the managed certs controller binary, then a docker image with this binary, and pushes the image, for dev
release: release-ci clean

# Builds the managed certs controller binary, then a docker image with this binary, and pushes the image, for continuous integration
release-ci: build-binary-in-docker run-test-in-docker docker
	make -C http-hello

run-e2e-in-docker: docker-runner-builder auth-configure-docker
	docker run -v `pwd`:${runner_path} \
		-v ${CLOUD_SDK_ROOT}:${CLOUD_SDK_ROOT} \
		-v ${CLOUD_CONFIG}:/root/.config/gcloud \
		-v ${KUBECONFIG}:/root/.kube/config \
		-v ${ARTIFACTS}:/tmp/artifacts \
		${runner_image}:latest bash -c 'cd ${runner_path} && make e2e DNS_ZONE=${DNS_ZONE} CLOUD_SDK_ROOT=${CLOUD_SDK_ROOT}'

run-test-in-docker: docker-runner-builder
	docker run -v `pwd`:${runner_path} ${runner_image}:latest bash -c 'cd ${runner_path} && make test'

test:
	godep go test ./pkg/... -cover

.PHONY: all auth-configure-docker build-binary build-binary-in-docker build-dev clean cross deps docker docker-runner-builder e2e release release-ci run-e2e-in-docker run-test-in-docker test
