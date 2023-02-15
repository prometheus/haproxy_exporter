## 0.15.0 / 2023-02-15

* [FEATURE] Add metric for idle time percentage #236 #255
* [ENHANCEMENT] Dependency updates #252 #253 #254

This is the **FINAL RELEASE** of the standalone HAProxy exporter.
All supported versions of HAProxy now have Prometheus metrics [built in](https://github.com/prometheus/haproxy_exporter#official-prometheus-exporter).
Please transition to using the built-in support as soon as possible.

## 0.14.0 / 2022-11-29

* [SECURITY] Update Exporter Toolkit (CVE-2022-46146) #251
* [FEATURE] Support multiple Listen Addresses and systemd socket activation #251

## 0.13.0 / 2021-11-26

* [FEATURE] Add TLS and Basic authentication #205
* [ENHANCEMENT] Added average over last 1024 requests metrics to server metric type #196
* [BUGFIX] Fix docker images architecture and publish ppc64le & s390x images #211

## 0.12.0 / 2020-12-09

* [ENHANCEMENT] Add --version flag #189
* [BUGFIX] Use newest Go version to fix random panic in the runtime
* [BUGFIX] Fix typos in log messages #188 #191

## 0.11.0 / 2020-06-21

* [CHANGE] Switch logging to go-kit #171
* [CHANGE] Fix metric types #182
* [CHANGE] Fix unit of time metric #183
* [FEATURE] Add filtering on server status #160
* [ENHANCEMENT] Add compression and server selection metrics #154
* [ENHANCEMENT] Add client/server abort metrics #167
* [ENHANCEMENT] Add version info metric (when using UNIX sockets) #180

Note: This release fixes the metric types of counters and renames the following metrics:

* `haproxy_exporter_csv_parse_failures` -> `haproxy_exporter_csv_parse_failures_total`
* `haproxy_exporter_total_scrapes` -> `haproxy_exporter_scrapes_total`
* `haproxy_server_check_duration_milliseconds` -> `haproxy_server_check_duration_seconds`

## 0.10.0 / 2019-01-15

* [ENHANCEMENT] Convert metrics collection to Const metrics #139
* [BUGFIX] Fix silent dropping of metrics for older versions of haproxy #139

## 0.9.0 / 2018-01-23

* [CHANGE] Rename `*_connections_total` to `*_sessions_total` following the changes in HAProxy 1.7
* [ENHANCEMENT] Add new `haproxy_server_connections_total` metric
* [ENHANCEMENT] Add new `--haproxy.ssl-verify` flag
* [BUGFIX] Convert latency metrics to correct unit.

## 0.8.0 / 2017-08-24

* [CHANGE] New flag handling (double dashs are required)
* [FEATURE] Add metric for session limit.
* [FEATURE] Add metrics for average HTTP request latency

## 0.7.1 / 2016-10-12

* [BUGFIX] Fix timeout behavior when reusing HTTP connections
* [BUGFIX] Remove usage of undocumented golang type assertion behavior

## 0.7.0 / 2016-06-08

* [FEATURE] Add support for unix sockets

## 0.6.0 / 2016-05-13

* [CHANGE] Use new build process, changes the structure of the tarball.
* [FIX] Abort on non-200 status code from HAProxy.
* [ENHANCEMENT] Add -version flag and version metric.
* [ENHANCEMENT] Add chkfail and downtime server metrics.
* [ENHANCEMENT] Remove locks and unnecessary channel communication.

## 0.5.2 / 2016-04-05

* [FIX] Limit graceful CSV error handling to parse errors

## 0.5.1 / 2016-03-31

* [FIX] Handle invalid CSV lines gracefully

## 0.5.0 / 2015-12-23

* [CHANGE] New Dockerfile
* [ENHANCEMENT] Export haproxy_check_duration_milliseconds
* [ENHANCEMENT] Export haproxy_limit_sessions
* [ENHANCEMENT] Export haproxy_limit_session_rate
* [ENHANCEMENT] Allow complete deactivation of server metrics
* [ENHANCEMENT] Use common prometheus logging
* [FIX] Fix status field parsing of servers in MAINT status
