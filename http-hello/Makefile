all: build docker clean

TAG?=dev
REGISTRY?=eu.gcr.io/managed-certs-gke
NAME=http-hello
DOCKER_IMAGE=${REGISTRY}/${NAME}:${TAG}

build:
	CGO_ENABLED=0 GOOS=linux go build -o ${NAME} -ldflags '-extldflags "-static"'

clean:
	rm -f ${NAME}

docker: build
	docker build --pull -t ${DOCKER_IMAGE} .
	docker push ${DOCKER_IMAGE}

docker-ci: build
	docker build --pull -t ${DOCKER_IMAGE} .
	gcloud auth activate-service-account --key-file=/etc/service-account/service-account.json
	gcloud auth configure-docker
	docker push ${DOCKER_IMAGE}

release: docker clean

release-ci: docker-ci

.PHONY: all build clean docker docker-ci release release-ci
