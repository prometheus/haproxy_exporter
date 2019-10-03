// Copyright 2018 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"crypto/tls"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	_ "net/http/pprof"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promlog"
	"github.com/prometheus/common/promlog/flag"
	"github.com/prometheus/common/version"
	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	namespace = "haproxy" // For Prometheus metrics.

	// HAProxy 1.4
	// # pxname,svname,qcur,qmax,scur,smax,slim,stot,bin,bout,dreq,dresp,ereq,econ,eresp,wretr,wredis,status,weight,act,bck,chkfail,chkdown,lastchg,downtime,qlimit,pid,iid,sid,throttle,lbtot,tracked,type,rate,rate_lim,rate_max,check_status,check_code,check_duration,hrsp_1xx,hrsp_2xx,hrsp_3xx,hrsp_4xx,hrsp_5xx,hrsp_other,hanafail,req_rate,req_rate_max,req_tot,cli_abrt,srv_abrt,
	// HAProxy 1.5
	// pxname,svname,qcur,qmax,scur,smax,slim,stot,bin,bout,dreq,dresp,ereq,econ,eresp,wretr,wredis,status,weight,act,bck,chkfail,chkdown,lastchg,downtime,qlimit,pid,iid,sid,throttle,lbtot,tracked,type,rate,rate_lim,rate_max,check_status,check_code,check_duration,hrsp_1xx,hrsp_2xx,hrsp_3xx,hrsp_4xx,hrsp_5xx,hrsp_other,hanafail,req_rate,req_rate_max,req_tot,cli_abrt,srv_abrt,comp_in,comp_out,comp_byp,comp_rsp,lastsess,
	// HAProxy 1.5.19
	// pxname,svname,qcur,qmax,scur,smax,slim,stot,bin,bout,dreq,dresp,ereq,econ,eresp,wretr,wredis,status,weight,act,bck,chkfail,chkdown,lastchg,downtime,qlimit,pid,iid,sid,throttle,lbtot,tracked,type,rate,rate_lim,rate_max,check_status,check_code,check_duration,hrsp_1xx,hrsp_2xx,hrsp_3xx,hrsp_4xx,hrsp_5xx,hrsp_other,hanafail,req_rate,req_rate_max,req_tot,cli_abrt,srv_abrt,comp_in,comp_out,comp_byp,comp_rsp,lastsess,last_chk,last_agt,qtime,ctime,rtime,ttime,
	// HAProxy 1.7
	// pxname,svname,qcur,qmax,scur,smax,slim,stot,bin,bout,dreq,dresp,ereq,econ,eresp,wretr,wredis,status,weight,act,bck,chkfail,chkdown,lastchg,downtime,qlimit,pid,iid,sid,throttle,lbtot,tracked,type,rate,rate_lim,rate_max,check_status,check_code,check_duration,hrsp_1xx,hrsp_2xx,hrsp_3xx,hrsp_4xx,hrsp_5xx,hrsp_other,hanafail,req_rate,req_rate_max,req_tot,cli_abrt,srv_abrt,comp_in,comp_out,comp_byp,comp_rsp,lastsess,last_chk,last_agt,qtime,ctime,rtime,ttime,agent_status,agent_code,agent_duration,check_desc,agent_desc,check_rise,check_fall,check_health,agent_rise,agent_fall,agent_health,addr,cookie,mode,algo,conn_rate,conn_rate_max,conn_tot,intercepted,dcon,dses
	minimumCsvFieldCount = 33
	statusField          = 17
	qtimeMsField         = 58
	ctimeMsField         = 59
	rtimeMsField         = 60
	ttimeMsField         = 61
)

var (
	frontendLabelNames = []string{"frontend"}
	backendLabelNames  = []string{"backend"}
	serverLabelNames   = []string{"backend", "server"}
)

func newFrontendMetric(metricName string, docString string, constLabels prometheus.Labels) *prometheus.Desc {
	return prometheus.NewDesc(prometheus.BuildFQName(namespace, "frontend", metricName), docString, frontendLabelNames, constLabels)
}

func newBackendMetric(metricName string, docString string, constLabels prometheus.Labels) *prometheus.Desc {
	return prometheus.NewDesc(prometheus.BuildFQName(namespace, "backend", metricName), docString, backendLabelNames, constLabels)
}

func newServerMetric(metricName string, docString string, constLabels prometheus.Labels) *prometheus.Desc {
	return prometheus.NewDesc(prometheus.BuildFQName(namespace, "server", metricName), docString, serverLabelNames, constLabels)
}

type metrics map[int]*prometheus.Desc

func (m metrics) String() string {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	s := make([]string, len(keys))
	for i, k := range keys {
		s[i] = strconv.Itoa(k)
	}
	return strings.Join(s, ",")
}

var (
	serverMetrics = metrics{
		2:  newServerMetric("current_queue", "Current number of queued requests assigned to this server.", nil),
		3:  newServerMetric("max_queue", "Maximum observed number of queued requests assigned to this server.", nil),
		4:  newServerMetric("current_sessions", "Current number of active sessions.", nil),
		5:  newServerMetric("max_sessions", "Maximum observed number of active sessions.", nil),
		6:  newServerMetric("limit_sessions", "Configured session limit.", nil),
		7:  newServerMetric("sessions_total", "Total number of sessions.", nil),
		8:  newServerMetric("bytes_in_total", "Current total of incoming bytes.", nil),
		9:  newServerMetric("bytes_out_total", "Current total of outgoing bytes.", nil),
		13: newServerMetric("connection_errors_total", "Total of connection errors.", nil),
		14: newServerMetric("response_errors_total", "Total of response errors.", nil),
		15: newServerMetric("retry_warnings_total", "Total of retry warnings.", nil),
		16: newServerMetric("redispatch_warnings_total", "Total of redispatch warnings.", nil),
		17: newServerMetric("up", "Current health status of the server (1 = UP, 0 = DOWN).", nil),
		18: newServerMetric("weight", "Current weight of the server.", nil),
		21: newServerMetric("check_failures_total", "Total number of failed health checks.", nil),
		24: newServerMetric("downtime_seconds_total", "Total downtime in seconds.", nil),
		30: newServerMetric("server_selected_total", "Total number of times a server was selected, either for new sessions, or when re-dispatching.", nil),
		33: newServerMetric("current_session_rate", "Current number of sessions per second over last elapsed second.", nil),
		35: newServerMetric("max_session_rate", "Maximum observed number of sessions per second.", nil),
		38: newServerMetric("check_duration_milliseconds", "Previously run health check duration, in milliseconds", nil),
		39: newServerMetric("http_responses_total", "Total of HTTP responses.", prometheus.Labels{"code": "1xx"}),
		40: newServerMetric("http_responses_total", "Total of HTTP responses.", prometheus.Labels{"code": "2xx"}),
		41: newServerMetric("http_responses_total", "Total of HTTP responses.", prometheus.Labels{"code": "3xx"}),
		42: newServerMetric("http_responses_total", "Total of HTTP responses.", prometheus.Labels{"code": "4xx"}),
		43: newServerMetric("http_responses_total", "Total of HTTP responses.", prometheus.Labels{"code": "5xx"}),
		44: newServerMetric("http_responses_total", "Total of HTTP responses.", prometheus.Labels{"code": "other"}),
	}

	frontendMetrics = metrics{
		4:  newFrontendMetric("current_sessions", "Current number of active sessions.", nil),
		5:  newFrontendMetric("max_sessions", "Maximum observed number of active sessions.", nil),
		6:  newFrontendMetric("limit_sessions", "Configured session limit.", nil),
		7:  newFrontendMetric("sessions_total", "Total number of sessions.", nil),
		8:  newFrontendMetric("bytes_in_total", "Current total of incoming bytes.", nil),
		9:  newFrontendMetric("bytes_out_total", "Current total of outgoing bytes.", nil),
		10: newFrontendMetric("requests_denied_total", "Total of requests denied for security.", nil),
		12: newFrontendMetric("request_errors_total", "Total of request errors.", nil),
		33: newFrontendMetric("current_session_rate", "Current number of sessions per second over last elapsed second.", nil),
		34: newFrontendMetric("limit_session_rate", "Configured limit on new sessions per second.", nil),
		35: newFrontendMetric("max_session_rate", "Maximum observed number of sessions per second.", nil),
		39: newFrontendMetric("http_responses_total", "Total of HTTP responses.", prometheus.Labels{"code": "1xx"}),
		40: newFrontendMetric("http_responses_total", "Total of HTTP responses.", prometheus.Labels{"code": "2xx"}),
		41: newFrontendMetric("http_responses_total", "Total of HTTP responses.", prometheus.Labels{"code": "3xx"}),
		42: newFrontendMetric("http_responses_total", "Total of HTTP responses.", prometheus.Labels{"code": "4xx"}),
		43: newFrontendMetric("http_responses_total", "Total of HTTP responses.", prometheus.Labels{"code": "5xx"}),
		44: newFrontendMetric("http_responses_total", "Total of HTTP responses.", prometheus.Labels{"code": "other"}),
		48: newFrontendMetric("http_requests_total", "Total HTTP requests.", nil),
		51: newFrontendMetric("compressor_bytes_in_total", "Number of HTTP response bytes fed to the compressor", nil),
		52: newFrontendMetric("compressor_bytes_out_total", "Number of HTTP response bytes emitted by the compressor", nil),
		53: newFrontendMetric("compressor_bytes_bypassed_total", "Number of bytes that bypassed the HTTP compressor", nil),
		54: newFrontendMetric("http_responses_compressed_total", "Number of HTTP responses that were compressed", nil),
		79: newFrontendMetric("connections_total", "Total number of connections", nil),
	}
	backendMetrics = metrics{
		2:  newBackendMetric("current_queue", "Current number of queued requests not assigned to any server.", nil),
		3:  newBackendMetric("max_queue", "Maximum observed number of queued requests not assigned to any server.", nil),
		4:  newBackendMetric("current_sessions", "Current number of active sessions.", nil),
		5:  newBackendMetric("max_sessions", "Maximum observed number of active sessions.", nil),
		6:  newBackendMetric("limit_sessions", "Configured session limit.", nil),
		7:  newBackendMetric("sessions_total", "Total number of sessions.", nil),
		8:  newBackendMetric("bytes_in_total", "Current total of incoming bytes.", nil),
		9:  newBackendMetric("bytes_out_total", "Current total of outgoing bytes.", nil),
		13: newBackendMetric("connection_errors_total", "Total of connection errors.", nil),
		14: newBackendMetric("response_errors_total", "Total of response errors.", nil),
		15: newBackendMetric("retry_warnings_total", "Total of retry warnings.", nil),
		16: newBackendMetric("redispatch_warnings_total", "Total of redispatch warnings.", nil),
		17: newBackendMetric("up", "Current health status of the backend (1 = UP, 0 = DOWN).", nil),
		18: newBackendMetric("weight", "Total weight of the servers in the backend.", nil),
		19: newBackendMetric("current_server", "Current number of active servers", nil),
		30: newBackendMetric("server_selected_total", "Total number of times a server was selected, either for new sessions, or when re-dispatching.", nil),
		33: newBackendMetric("current_session_rate", "Current number of sessions per second over last elapsed second.", nil),
		35: newBackendMetric("max_session_rate", "Maximum number of sessions per second.", nil),
		39: newBackendMetric("http_responses_total", "Total of HTTP responses.", prometheus.Labels{"code": "1xx"}),
		40: newBackendMetric("http_responses_total", "Total of HTTP responses.", prometheus.Labels{"code": "2xx"}),
		41: newBackendMetric("http_responses_total", "Total of HTTP responses.", prometheus.Labels{"code": "3xx"}),
		42: newBackendMetric("http_responses_total", "Total of HTTP responses.", prometheus.Labels{"code": "4xx"}),
		43: newBackendMetric("http_responses_total", "Total of HTTP responses.", prometheus.Labels{"code": "5xx"}),
		44: newBackendMetric("http_responses_total", "Total of HTTP responses.", prometheus.Labels{"code": "other"}),
		51: newBackendMetric("compressor_bytes_in_total", "Number of HTTP response bytes fed to the compressor", nil),
		52: newBackendMetric("compressor_bytes_out_total", "Number of HTTP response bytes emitted by the compressor", nil),
		53: newBackendMetric("compressor_bytes_bypassed_total", "Number of bytes that bypassed the HTTP compressor", nil),
		54: newBackendMetric("http_responses_compressed_total", "Number of HTTP responses that were compressed", nil),
		58: newBackendMetric("http_queue_time_average_seconds", "Avg. HTTP queue time for last 1024 successful connections.", nil),
		59: newBackendMetric("http_connect_time_average_seconds", "Avg. HTTP connect time for last 1024 successful connections.", nil),
		60: newBackendMetric("http_response_time_average_seconds", "Avg. HTTP response time for last 1024 successful connections.", nil),
		61: newBackendMetric("http_total_time_average_seconds", "Avg. HTTP total time for last 1024 successful connections.", nil),
	}

	haproxyUp = prometheus.NewDesc(prometheus.BuildFQName(namespace, "", "up"), "Was the last scrape of haproxy successful.", nil, nil)
)

// Exporter collects HAProxy stats from the given URI and exports them using
// the prometheus metrics package.
type Exporter struct {
	URI   string
	mutex sync.RWMutex
	fetch func() (io.ReadCloser, error)

	up                             prometheus.Gauge
	totalScrapes, csvParseFailures prometheus.Counter
	serverMetrics                  map[int]*prometheus.Desc
	logger                         log.Logger
}

// NewExporter returns an initialized Exporter.
func NewExporter(uri string, sslVerify bool, selectedServerMetrics map[int]*prometheus.Desc, timeout time.Duration, logger log.Logger) (*Exporter, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}

	var fetch func() (io.ReadCloser, error)
	switch u.Scheme {
	case "http", "https", "file":
		fetch = fetchHTTP(uri, sslVerify, timeout)
	case "unix":
		fetch = fetchUnix(u, timeout)
	default:
		return nil, fmt.Errorf("unsupported scheme: %q", u.Scheme)
	}

	return &Exporter{
		URI:   uri,
		fetch: fetch,
		up: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "up",
			Help:      "Was the last scrape of haproxy successful.",
		}),
		totalScrapes: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "exporter_total_scrapes",
			Help:      "Current total HAProxy scrapes.",
		}),
		csvParseFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "exporter_csv_parse_failures",
			Help:      "Number of errors while parsing CSV.",
		}),
		serverMetrics: selectedServerMetrics,
		logger:        logger,
	}, nil
}

// Describe describes all the metrics ever exported by the HAProxy exporter. It
// implements prometheus.Collector.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	for _, m := range frontendMetrics {
		ch <- m
	}
	for _, m := range backendMetrics {
		ch <- m
	}
	for _, m := range e.serverMetrics {
		ch <- m
	}
	ch <- haproxyUp
	ch <- e.totalScrapes.Desc()
	ch <- e.csvParseFailures.Desc()
}

// Collect fetches the stats from configured HAProxy location and delivers them
// as Prometheus metrics. It implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.mutex.Lock() // To protect metrics from concurrent collects.
	defer e.mutex.Unlock()

	up := e.scrape(ch)

	ch <- prometheus.MustNewConstMetric(haproxyUp, prometheus.GaugeValue, up)
	ch <- e.totalScrapes
	ch <- e.csvParseFailures
}

func fetchHTTP(uri string, sslVerify bool, timeout time.Duration) func() (io.ReadCloser, error) {
	tr := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: !sslVerify}}
	client := http.Client{
		Timeout:   timeout,
		Transport: tr,
	}

	return func() (io.ReadCloser, error) {
		resp, err := client.Get(uri)
		if err != nil {
			return nil, err
		}
		if !(resp.StatusCode >= 200 && resp.StatusCode < 300) {
			resp.Body.Close()
			return nil, fmt.Errorf("HTTP status %d", resp.StatusCode)
		}
		return resp.Body, nil
	}
}

func fetchUnix(u *url.URL, timeout time.Duration) func() (io.ReadCloser, error) {
	return func() (io.ReadCloser, error) {
		f, err := net.DialTimeout("unix", u.Path, timeout)
		if err != nil {
			return nil, err
		}
		if err := f.SetDeadline(time.Now().Add(timeout)); err != nil {
			f.Close()
			return nil, err
		}
		cmd := "show stat\n"
		n, err := io.WriteString(f, cmd)
		if err != nil {
			f.Close()
			return nil, err
		}
		if n != len(cmd) {
			f.Close()
			return nil, errors.New("write error")
		}
		return f, nil
	}
}

func (e *Exporter) scrape(ch chan<- prometheus.Metric) (up float64) {
	e.totalScrapes.Inc()

	body, err := e.fetch()
	if err != nil {
		level.Error(e.logger).Log("msg", "Can't scrape HAProxy", "err", err)
		return 0
	}
	defer body.Close()

	reader := csv.NewReader(body)
	reader.TrailingComma = true
	reader.Comment = '#'

loop:
	for {
		row, err := reader.Read()
		switch err {
		case nil:
		case io.EOF:
			break loop
		default:
			if _, ok := err.(*csv.ParseError); ok {
				level.Error(e.logger).Log("msg", "Can't read CSV", "err", err)
				e.csvParseFailures.Inc()
				continue loop
			}
			level.Error(e.logger).Log("msg", "Unexpected error while reading CSV", "err", err)
			return 0
		}
		e.parseRow(row, ch)
	}
	return 1
}

func (e *Exporter) parseRow(csvRow []string, ch chan<- prometheus.Metric) {
	if len(csvRow) < minimumCsvFieldCount {
		level.Error(e.logger).Log("msg", "Parser received unexpected number of CSV fileds", "min", minimumCsvFieldCount, "received", len(csvRow))
		e.csvParseFailures.Inc()
		return
	}

	pxname, svname, typ := csvRow[0], csvRow[1], csvRow[32]

	const (
		frontend = "0"
		backend  = "1"
		server   = "2"
	)

	switch typ {
	case frontend:
		e.exportCsvFields(frontendMetrics, csvRow, ch, pxname)
	case backend:
		e.exportCsvFields(backendMetrics, csvRow, ch, pxname)
	case server:
		e.exportCsvFields(e.serverMetrics, csvRow, ch, pxname, svname)
	}
}

func parseStatusField(value string) int64 {
	switch value {
	case "UP", "UP 1/3", "UP 2/3", "OPEN", "no check":
		return 1
	case "DOWN", "DOWN 1/2", "NOLB", "MAINT":
		return 0
	}
	return 0
}

func (e *Exporter) exportCsvFields(metrics map[int]*prometheus.Desc, csvRow []string, ch chan<- prometheus.Metric, labels ...string) {
	for fieldIdx, metric := range metrics {
		if fieldIdx > len(csvRow)-1 {
			// We can't break here because we are not looping over the fields in sorted order.
			continue
		}
		valueStr := csvRow[fieldIdx]
		if valueStr == "" {
			continue
		}

		var err error = nil
		var value float64
		var valueInt int64

		switch fieldIdx {
		case statusField:
			valueInt = parseStatusField(valueStr)
			value = float64(valueInt)
		case qtimeMsField, ctimeMsField, rtimeMsField, ttimeMsField:
			value, err = strconv.ParseFloat(valueStr, 64)
			value /= 1000
		default:
			valueInt, err = strconv.ParseInt(valueStr, 10, 64)
			value = float64(valueInt)
		}
		if err != nil {
			level.Error(e.logger).Log("msg", "Can't parse CSV field value", "value", valueStr, "err", err)
			e.csvParseFailures.Inc()
			continue
		}
		ch <- prometheus.MustNewConstMetric(metric, prometheus.GaugeValue, value, labels...)
	}
}

// filterServerMetrics returns the set of server metrics specified by the comma
// separated filter.
func filterServerMetrics(filter string) (map[int]*prometheus.Desc, error) {
	metrics := map[int]*prometheus.Desc{}
	if len(filter) == 0 {
		return metrics, nil
	}

	selected := map[int]struct{}{}
	for _, f := range strings.Split(filter, ",") {
		field, err := strconv.Atoi(f)
		if err != nil {
			return nil, fmt.Errorf("invalid server metric field number: %v", f)
		}
		selected[field] = struct{}{}
	}

	for field, metric := range serverMetrics {
		if _, ok := selected[field]; ok {
			metrics[field] = metric
		}
	}
	return metrics, nil
}

func main() {
	const pidFileHelpText = `Path to HAProxy pid file.

	If provided, the standard process metrics get exported for the HAProxy
	process, prefixed with 'haproxy_process_...'. The haproxy_process exporter
	needs to have read access to files owned by the HAProxy process. Depends on
	the availability of /proc.

	https://prometheus.io/docs/instrumenting/writing_clientlibs/#process-metrics.`

	var (
		listenAddress             = kingpin.Flag("web.listen-address", "Address to listen on for web interface and telemetry.").Default(":9101").String()
		metricsPath               = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()
		haProxyScrapeURI          = kingpin.Flag("haproxy.scrape-uri", "URI on which to scrape HAProxy.").Default("http://localhost/;csv").String()
		haProxySSLVerify          = kingpin.Flag("haproxy.ssl-verify", "Flag that enables SSL certificate verification for the scrape URI").Default("true").Bool()
		haProxyServerMetricFields = kingpin.Flag("haproxy.server-metric-fields", "Comma-separated list of exported server metrics. See http://cbonte.github.io/haproxy-dconv/configuration-1.5.html#9.1").Default(serverMetrics.String()).String()
		haProxyTimeout            = kingpin.Flag("haproxy.timeout", "Timeout for trying to get stats from HAProxy.").Default("5s").Duration()
		haProxyPidFile            = kingpin.Flag("haproxy.pid-file", pidFileHelpText).Default("").String()
	)

	promlogConfig := &promlog.Config{}
	flag.AddFlags(kingpin.CommandLine, promlogConfig)
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()
	logger := promlog.New(promlogConfig)

	selectedServerMetrics, err := filterServerMetrics(*haProxyServerMetricFields)
	if err != nil {
		level.Error(logger).Log("msg", "Error filtering server metrics", "err", err)
		os.Exit(1)
	}

	level.Info(logger).Log("msg", "Starting haproxy_exporter", "version", version.Info())
	level.Info(logger).Log("msg", "Build context", "context", version.BuildContext())

	exporter, err := NewExporter(*haProxyScrapeURI, *haProxySSLVerify, selectedServerMetrics, *haProxyTimeout, logger)
	if err != nil {
		level.Error(logger).Log("msg", "Error creating an exporter", "err", err)
		os.Exit(1)
	}
	prometheus.MustRegister(exporter)
	prometheus.MustRegister(version.NewCollector("haproxy_exporter"))

	if *haProxyPidFile != "" {
		procExporter := prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{
			PidFn: func() (int, error) {
				content, err := ioutil.ReadFile(*haProxyPidFile)
				if err != nil {
					return 0, fmt.Errorf("can't read pid file: %s", err)
				}
				value, err := strconv.Atoi(strings.TrimSpace(string(content)))
				if err != nil {
					return 0, fmt.Errorf("can't parse pid file: %s", err)
				}
				return value, nil
			},
			Namespace: namespace,
		})
		prometheus.MustRegister(procExporter)
	}

	level.Info(logger).Log("msg", "Listening on address", "address", *listenAddress)
	http.Handle(*metricsPath, promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
             <head><title>Haproxy Exporter</title></head>
             <body>
             <h1>Haproxy Exporter</h1>
             <p><a href='` + *metricsPath + `'>Metrics</a></p>
             </body>
             </html>`))
	})
	if err := http.ListenAndServe(*listenAddress, nil); err != nil {
		level.Error(logger).Log("msg", "Error starting HTTP server", "err", err)
		os.Exit(1)
	}
}
