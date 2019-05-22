all: gofmt vet build-binary-in-docker run-test-in-docker clean cross

TAG ?= ${USER}-dev
REGISTRY ?= eu.gcr.io/managed-certs-gke
KUBECONFIG ?= ${HOME}/.kube/config
KUBERNETES_PROVIDER ?= gke
ARTIFACTS ?= /tmp/artifacts
CLOUD_CONFIG ?= $(shell gcloud info --format="value(config.paths.global_config_dir)")
CLOUD_SDK_ROOT ?= $(shell gcloud info --format="value(installation.sdk_root)")
PROJECT_ID ?= $(shell gcloud config list --format="value(core.project)")
DNS_ZONE ?= managedcertsgke
PLATFORM ?= GKE

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

# Builds the managed-certificate-controller binary using a docker runner image
build-binary-in-docker: docker-runner-builder
	docker run -v `pwd`:${runner_path} ${runner_image}:latest bash -c 'cd ${runner_path} && make build-binary GIT_COMMIT=${GIT_COMMIT} VERSION=${VERSION}'

clean:
	rm -f ${name}

# Checks if Google criteria for releasing code as OSS are met
cross:
	/google/data/ro/teams/opensource/cross .

deps:
	go get github.com/tools/godep

# Builds and pushes a docker image with managed-certificate-controller binary
docker: auth-configure-docker
	until docker build --pull -t ${REGISTRY}/${name}:${TAG} -t ${REGISTRY}/${name}:${VERSION} .; do \
		echo "Building managed-cetrificate-controller image failed, retrying in 10 seconds..." && sleep 10; \
	done
	until docker push ${REGISTRY}/${name}:${TAG}; do \
		echo "Pushing managed-certificate-controller image failed, retrying in 10 seconds..." && sleep 10; \
	done
	until docker push ${REGISTRY}/${name}:${VERSION}; do \
		echo "Pushing managed-certificate-controller image failed, retrying in 10 seconds..." && sleep 10; \
	done

# Builds a runner image, i. e. an image used to build a managed-certificate-controller binary and to run its tests.
docker-runner-builder:
	until docker build -t ${runner_image} runner; do \
		echo "Building runner image failed, retrying in 10 seconds..." && sleep 10; \
	done

e2e:
	dest=/tmp/artifacts; \
	rm -rf $${dest}/* && mkdir -p $${dest} && \
	{ \
		CLOUD_SDK_ROOT=${CLOUD_SDK_ROOT} \
		KUBECONFIG=${KUBECONFIG} \
		KUBERNETES_PROVIDER=${KUBERNETES_PROVIDER} \
		PROJECT_ID=${PROJECT_ID} \
		DNS_ZONE=${DNS_ZONE} \
		PLATFORM=${PLATFORM} \
		TAG=${TAG} \
		godep go test ./e2e/... -test.timeout=60m \
			-logtostderr=false -alsologtostderr=true -v -log_dir=$${dest} \
			> $${dest}/e2e.out.txt && exitcode=$${?} || exitcode=$${?} ; \
	} && cat $${dest}/e2e.out.txt | go-junit-report > $${dest}/junit_01.xml && exit $${exitcode}

# Formats go source code with gofmt
gofmt:
	gofmt -w main.go
	find . -mindepth 1 -maxdepth 1 -name Godeps -o -name vendor -prune -o -type d -print | xargs gofmt -w

# Builds the managed certs controller binary, then a docker image with this binary, and pushes the image, for dev
release: release-ci clean

# Builds the managed certs controller binary, then a docker image with this binary, and pushes the image, for continuous integration
release-ci: build-binary-in-docker run-test-in-docker docker

run-e2e-in-docker: docker-runner-builder auth-configure-docker
	docker run -v `pwd`:${runner_path} \
		-v ${CLOUD_SDK_ROOT}:${CLOUD_SDK_ROOT} \
		-v ${CLOUD_CONFIG}:/root/.config/gcloud \
		-v ${CLOUD_CONFIG}:/root/.config/gcloud-staging \
		-v ${KUBECONFIG}:/root/.kube/config \
		-v ${ARTIFACTS}:/tmp/artifacts \
		${runner_image}:latest bash -c 'cd ${runner_path} && make e2e \
		DNS_ZONE=${DNS_ZONE} CLOUD_SDK_ROOT=${CLOUD_SDK_ROOT} PROJECT_ID=${PROJECT_ID} PLATFORM=${PLATFORM} TAG=${TAG}'

run-test-in-docker: docker-runner-builder
	docker run -v `pwd`:${runner_path} ${runner_image}:latest bash -c 'cd ${runner_path} && make test'

test:
	godep go test ./pkg/... -cover

vet:
	godep go vet ./...

.PHONY: all auth-configure-docker build-binary build-binary-in-docker build-dev clean cross deps docker docker-runner-builder e2e release release-ci run-e2e-in-docker run-test-in-docker test vet
