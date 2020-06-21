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
