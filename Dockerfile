FROM golang

COPY managed-certs-controller /
COPY run.sh /

CMD /run.sh
