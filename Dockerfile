ARG ARCH="amd64"
ARG OS="linux"
FROM        quay.io/prometheus/busybox:latest
LABEL maintainer="The Prometheus Authors <prometheus-developers@googlegroups.com>"

ARG ARCH="amd64"
ARG OS="linux"
COPY .build/${OS}-${ARCH}/haproxy_exporter /bin/haproxy_exporter

ENTRYPOINT ["/bin/haproxy_exporter"]
EXPOSE     9101
