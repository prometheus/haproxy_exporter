# HAProxy Exporter for Prometheus

This is a simple server that periodically scrapes HAProxy stats and exports them via HTTP/JSON for Prometheus
consumption.

To run it:

```bash
go run haproxy_exporter [flags]
```

Help on flags:
```bash
go run haproxy_exporter --help
```

## Getting Started
  * The source code is periodically indexed: [Prometheus HAProxy Exporter Bridge](http://godoc.org/github.com/prometheus/haproxy_exporter).
  * All of the core developers are accessible via the [Prometheus Developers Mailinglist](https://groups.google.com/forum/?fromgroups#!forum/prometheus-developers).

## Testing

[![Build Status](https://travis-ci.org/prometheus/haproxy_exporter.png?branch=master)](https://travis-ci.org/prometheus/haproxy_exporter)

## Docker

To run the haproxy exporter as a Docker container, run:

    $ docker run -p 9101:9101 prom/haproxy-exporter -haproxy.scrape_uri="http://user:pass@haproxy.example.com/haproxy?stats;csv"
