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
