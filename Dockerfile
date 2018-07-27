FROM k8s.gcr.io/debian-base-amd64:0.3

ENV DEBIAN_FRONTEND noninteractive
RUN clean-install ca-certificates

ADD managed-certs-controller /managed-certs-controller
ADD run.sh /run.sh

CMD ./run.sh
