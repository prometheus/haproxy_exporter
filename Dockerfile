FROM        quay.io/prometheus/busybox:latest
MAINTAINER  The Prometheus Authors <prometheus-developers@googlegroups.com>

COPY haproxy_exporter /bin/haproxy_exporter

ENTRYPOINT ["/bin/haproxy_exporter"]
EXPOSE     9101
