# HAProxy Exporter for Prometheus

This is a simple server that scrapes HAProxy stats and exports them via HTTP for
Prometheus consumption.

## This exporter is retired

In all supported versions of HAProxy, the official source includes a Prometheus exporter module that can be built into your binary with a single flag during build time and offers a native Prometheus endpoint. For more information see [down below](#official-prometheus-exporter).

Please transition to using the built-in support as soon as possible.

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

Alternatively, provide the password through a file, so that it does not appear in the process
table or in the output of the ```/debug/pprof/cmdline``` profiling service:

```bash
echo '--haproxy.scrape-uri=http://user:pass@haproxy.example.com/haproxy?stats;csv' > args
haproxy_exporter @args
```

You can also scrape HTTPS URLs. Certificate validation is enabled by default, but
you can disable it using the `--no-haproxy.ssl-verify` flag:

```bash
haproxy_exporter --no-haproxy.ssl-verify --haproxy.scrape-uri="https://haproxy.example.com/haproxy?stats;csv"
```

If scraping a remote HAProxy must be done via an HTTP proxy, you can enable reading of the
standard [`$http_proxy` / `$https_proxy` / `$no_proxy` environment variables](https://pkg.go.dev/net/http#ProxyFromEnvironment) by using the
`--http.proxy-from-env` flag (these variables will be ignored otherwise):

```bash
export HTTP_PROXY="http://proxy:3128"
haproxy_exporter --http.proxy-from-env --haproxy.scrape-uri="http://haproxy.example.com/haproxy?stats;csv"
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
docker run -p 9101:9101 quay.io/prometheus/haproxy-exporter:latest --haproxy.scrape-uri="http://user:pass@haproxy.example.com/haproxy?stats;csv"
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

[![CircleCI](https://circleci.com/gh/prometheus/haproxy_exporter/tree/main.svg?style=shield)][circleci]

```bash
make test
```

[circleci]: https://circleci.com/gh/prometheus/haproxy_exporter

### TLS and basic authentication

The HAProxy Exporter supports TLS and basic authentication.

To use TLS and/or basic authentication, you need to pass a configuration file
using the `--web.config.file` parameter. The format of the file is described
[in the exporter-toolkit repository](https://github.com/prometheus/exporter-toolkit/blob/master/docs/web-configuration.md).

## License

Apache License 2.0, see [LICENSE](https://github.com/prometheus/haproxy_exporter/blob/main/LICENSE).

## Alternatives

### Official Prometheus exporter

As of 2.0.0, HAProxy includes a Prometheus exporter module that can be built into your binary during build time.
For HAProxy 2.4 and higher, pass the `USE_PROMEX` flag to `make`:

```bash
make TARGET=linux-glibc USE_PROMEX=1
```

Pre-built versions, including the [Docker image](https://hub.docker.com/_/haproxy), typically have this enabled already.

Once built, you can enable and configure the Prometheus endpoint from your `haproxy.cfg` file as a typical frontend:

```haproxy
frontend stats
    bind *:8404
    http-request use-service prometheus-exporter if { path /metrics }
    stats enable
    stats uri /stats
    stats refresh 10s
```

For more information, see [this official blog post](https://www.haproxy.com/blog/haproxy-exposes-a-prometheus-metrics-endpoint/).
