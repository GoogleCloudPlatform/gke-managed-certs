all: build

TAG=v0.0.1
REGISTRY=eu.gcr.io
NAME=certs-controller
DOCKER_IMAGE=${REGISTRY}/managed-certs-gke/${NAME}:${TAG}

deps:
	go get github.com/tools/godep

build: clean deps
	godep go build ./...
	godep go build -o ${NAME}

docker:
	docker build --pull -t ${DOCKER_IMAGE} .
	docker push ${DOCKER_IMAGE}

release: build docker

clean:
	rm -f ${NAME}

.PHONY: all deps build clean release
