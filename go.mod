module github.com/prometheus/haproxy_exporter

go 1.14

require (
	github.com/alecthomas/units v0.0.0-20201120081800-1786d5ef83d4 // indirect
	github.com/go-kit/kit v0.10.0
	github.com/prometheus/client_golang v1.9.0
	github.com/prometheus/common v0.18.0
	github.com/prometheus/exporter-toolkit v0.5.1
	// Pin to new version to fix windows/arm64 build.
	golang.org/x/sys v0.0.0-20211123173158-ef496fb156ab // indirect
	google.golang.org/protobuf v1.25.0 // indirect
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
)
