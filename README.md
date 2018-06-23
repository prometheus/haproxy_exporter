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
haproxy_exporter --no-haproxy.ssl-verify --haproxy.scrape-uri="https://haproxy.example.com/haproxy?stats;csv"
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

### Available metrics

| metric | description |
| -------|:-----------:|
| haproxy_backend_bytes_in_total | Current total of incoming bytes. |
| haproxy_backend_bytes_out_total | Current total of outgoing bytes. |
| haproxy_backend_connection_errors_total | Total of connection errors. |
| haproxy_backend_current_queue | Current number of queued requests not assigned to any server. |
| haproxy_backend_current_server | Current number of active servers |
| haproxy_backend_current_session_rate | Current number of sessions per second over last elapsed second. |
| haproxy_backend_current_sessions | Current number of active sessions. |
| haproxy_backend_http_connect_time_average_seconds | Avg. HTTP connect time for last 1024 successful connections. |
| haproxy_backend_http_queue_time_average_seconds | Avg. HTTP queue time for last 1024 successful connections. |
| haproxy_backend_http_response_time_average_seconds | Avg. HTTP response time for last 1024 successful connections. |
| haproxy_backend_http_responses_total | Total of HTTP responses. |
| haproxy_backend_http_total_time_average_seconds | Avg. HTTP total time for last 1024 successful connections. |
| haproxy_backend_limit_sessions | Configured session limit. |
| haproxy_backend_max_queue | Maximum observed number of queued requests not assigned to any server. |
| haproxy_backend_max_session_rate | Maximum number of sessions per second. |
| haproxy_backend_max_sessions | Maximum observed number of active sessions. |
| haproxy_backend_redispatch_warnings_total | Total of redispatch warnings. |
| haproxy_backend_response_errors_total | Total of response errors. |
| haproxy_backend_retry_warnings_total | Total of retry warnings. |
| haproxy_backend_sessions_total | Total number of sessions. |
| haproxy_backend_up | Current health status of the backend (1 = UP, 0 = DOWN). |
| haproxy_backend_weight | Total weight of the servers in the backend. |
| haproxy_exporter_build_info | A metric with a constant '1' value labeled by version, revision, branch, and goversion from which haproxy_exporter was built. |
| haproxy_exporter_csv_parse_failures | Number of errors while parsing CSV. |
| haproxy_exporter_total_scrapes | Current total HAProxy scrapes. |
| haproxy_frontend_bytes_in_total | Current total of incoming bytes. |
| haproxy_frontend_bytes_out_total | Current total of outgoing bytes. |
| haproxy_frontend_current_session_rate | Current number of sessions per second over last elapsed second. |
| haproxy_frontend_current_sessions | Current number of active sessions. |
| haproxy_frontend_http_requests_total | Total HTTP requests. |
| haproxy_frontend_http_responses_total | Total of HTTP responses. |
| haproxy_frontend_limit_session_rate | Configured limit on new sessions per second. |
| haproxy_frontend_limit_sessions | Configured session limit. |
| haproxy_frontend_max_session_rate | Maximum observed number of sessions per second. |
| haproxy_frontend_max_sessions | Maximum observed number of active sessions. |
| haproxy_frontend_request_errors_total | Total of request errors. |
| haproxy_frontend_requests_denied_total | Total of requests denied for security. |
| haproxy_frontend_sessions_total | Total number of sessions. |
| haproxy_server_bytes_in_total | Current total of incoming bytes. |
| haproxy_server_bytes_out_total | Current total of outgoing bytes. |
| haproxy_server_check_duration_milliseconds | Previously run health check duration, in milliseconds |
| haproxy_server_check_failures_total | Total number of failed health checks. |
| haproxy_server_connection_errors_total | Total of connection errors. |
| haproxy_server_current_queue | Current number of queued requests assigned to this server. |
| haproxy_server_current_session_rate | Current number of sessions per second over last elapsed second. |
| haproxy_server_current_sessions | Current number of active sessions. |
| haproxy_server_downtime_seconds_total | Total downtime in seconds. |
| haproxy_server_http_responses_total | Total of HTTP responses. |
| haproxy_server_max_queue | Maximum observed number of queued requests assigned to this server. |
| haproxy_server_max_session_rate | Maximum observed number of sessions per second. |
| haproxy_server_max_sessions | Maximum observed number of active sessions. |
| haproxy_server_redispatch_warnings_total | Total of redispatch warnings. |
| haproxy_server_response_errors_total | Total of response errors. |
| haproxy_server_retry_warnings_total | Total of retry warnings. |
| haproxy_server_sessions_total | Total number of sessions. |
| haproxy_server_up | Current health status of the server (1 = UP, 0 = DOWN). |
| haproxy_server_weight | Current weight of the server. |
| haproxy_up | Was the last scrape of haproxy successful. |
| http_request_duration_microseconds | The HTTP request latencies in microseconds. |
| http_request_size_bytes | The HTTP request sizes in bytes. |
| http_requests_total | Total number of HTTP requests made. |
| http_response_size_bytes | The HTTP response sizes in bytes. |

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
