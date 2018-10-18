all: build docker clean

TAG ?= dev
registry = eu.gcr.io/managed-certs-gke
name = http-hello
docker_image = ${registry}/${name}:${TAG}

build:
	CGO_ENABLED=0 GOOS=linux go build -o ${name} -ldflags '-extldflags "-static"'

clean:
	rm -f ${name}

docker: build
	docker build --pull -t ${docker_image} .
	docker push ${docker_image}

.PHONY: all build clean docker
