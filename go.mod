module github.com/prometheus/haproxy_exporter

go 1.14

require (
	github.com/go-kit/log v0.2.0
	github.com/prometheus/client_golang v1.11.0
	github.com/prometheus/common v0.32.1
	github.com/prometheus/exporter-toolkit v0.7.0
	// Pin to new version to fix windows/arm64 build.
	golang.org/x/sys v0.0.0-20211123173158-ef496fb156ab // indirect
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
)
