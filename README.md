# HAProxy Exporter for Prometheus

This is a simple server that scrapes HAProxy stats and exports them via HTTP for
Prometheus consumption.

## Getting Started

To run it:

```bash
./haproxy_exporter [flags]
```

Help on flags:

```bash
./haproxy_exporter --help
```

For more information check the [source code documentation][gdocs]. All of the
core developers are accessible via the Prometheus Developers [mailinglist][].

[gdocs]: http://godoc.org/github.com/prometheus/haproxy_exporter
[mailinglist]: https://groups.google.com/forum/?fromgroups#!forum/prometheus-developers

## Usage

### HTTP stats URL

Specify custom URLs for the HAProxy stats port using the `--haproxy.scrape-uri`
flag. For example, if you have set `stats uri /baz`,

```bash
haproxy_exporter --haproxy.scrape-uri="http://localhost:5000/baz?stats;csv"
```

Or to scrape a remote host:

```bash
haproxy_exporter --haproxy.scrape-uri="http://haproxy.example.com/haproxy?stats;csv"
```

Note that the `;csv` is mandatory (and needs to be quoted).

If your stats port is protected by [basic auth][], add the credentials to the
scrape URL:

```bash
haproxy_exporter  --haproxy.scrape-uri="http://user:pass@haproxy.example.com/haproxy?stats;csv"
```

You can also scrape HTTPS URLs. Certificate validation is enabled by default, but
you can disable it using the `--haproxy.ssl-verify=false` flag:

```bash
haproxy_exporter --haproxy.ssl-verify=false --haproxy.scrape-uri="https://haproxy.example.com/haproxy?stats;csv"
```

[basic auth]: https://cbonte.github.io/haproxy-dconv/configuration-1.6.html#4-stats%20auth

### Unix Sockets

As alternative to localhost HTTP a stats socket can be used. Enable the stats
socket in HAProxy with for example:


    stats socket /run/haproxy/admin.sock mode 660 level admin


The scrape URL uses the 'unix:' scheme:

```bash
haproxy_exporter --haproxy.scrape-uri=unix:/run/haproxy/admin.sock
```

### Docker

[![Docker Repository on Quay](https://quay.io/repository/prometheus/haproxy-exporter/status)][quay]
[![Docker Pulls](https://img.shields.io/docker/pulls/prom/haproxy-exporter.svg?maxAge=604800)][hub]

To run the haproxy exporter as a Docker container, run:

```bash
docker run -p 9101:9101 quay.io/prometheus/haproxy-exporter:v0.9.0 --haproxy.scrape-uri="http://user:pass@haproxy.example.com/haproxy?stats;csv"
```

[hub]: https://hub.docker.com/r/prom/haproxy-exporter/
[quay]: https://quay.io/repository/prometheus/haproxy-exporter

## Development

[![Go Report Card](https://goreportcard.com/badge/github.com/prometheus/haproxy_exporter)][goreportcard]
[![Code Climate](https://codeclimate.com/github/prometheus/haproxy_exporter/badges/gpa.svg)][codeclimate]

[goreportcard]: https://goreportcard.com/report/github.com/prometheus/haproxy_exporter
[codeclimate]: https://codeclimate.com/github/prometheus/haproxy_exporter

### Building

```bash
make build
```

### Testing

[![Build Status](https://travis-ci.org/prometheus/haproxy_exporter.png?branch=master)][travisci]
[![CircleCI](https://circleci.com/gh/prometheus/haproxy_exporter/tree/master.svg?style=shield)][circleci]

```bash
make test
```

[travisci]: https://travis-ci.org/prometheus/haproxy_exporter
[circleci]: https://circleci.com/gh/prometheus/haproxy_exporter

## License

Apache License 2.0, see [LICENSE](https://github.com/prometheus/haproxy_exporter/blob/master/LICENSE).
