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

## Custom HAProxy stats URL

Specify custom URLs for the HAProxy stats port using the `-haproxy.scrape-uri` flag. For example, if you have set `stats uri /baz`,

```bash
haproxy_exporter -haproxy.scrape-uri="http://user:pass@localhost:5000/baz?stats;csv"
```

Or to scrape a remote host

```bash
haproxy_exporter -haproxy.scrape-uri="http://haproxy.example.com/haproxy?stats;csv"
```

Note that the `;csv` is mandatory (and needs to be quoted).


## Basic Auth

If your stats port is protected by [basic auth](https://cbonte.github.io/haproxy-dconv/configuration-1.6.html#4-stats%20auth), add the credentials to the scrape URL:

```bash
haproxy_exporter  -haproxy.scrape-uri="http://user:pass@haproxy.example.com/haproxy?stats;csv"
```

## Docker

To run the haproxy exporter as a Docker container, run:

    $ docker run -p 9101:9101 prom/haproxy-exporter -haproxy.scrape-uri="http://user:pass@haproxy.example.com/haproxy?stats;csv"
