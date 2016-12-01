package main

import (
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	_ "net/http/pprof"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
)

const (
	namespace = "haproxy" // For Prometheus metrics.

	// HAProxy 1.4
	// # pxname,svname,qcur,qmax,scur,smax,slim,stot,bin,bout,dreq,dresp,ereq,econ,eresp,wretr,wredis,status,weight,act,bck,chkfail,chkdown,lastchg,downtime,qlimit,pid,iid,sid,throttle,lbtot,tracked,type,rate,rate_lim,rate_max,check_status,check_code,check_duration,hrsp_1xx,hrsp_2xx,hrsp_3xx,hrsp_4xx,hrsp_5xx,hrsp_other,hanafail,req_rate,req_rate_max,req_tot,cli_abrt,srv_abrt,
	// HAProxy 1.5
	// pxname,svname,qcur,qmax,scur,smax,slim,stot,bin,bout,dreq,dresp,ereq,econ,eresp,wretr,wredis,status,weight,act,bck,chkfail,chkdown,lastchg,downtime,qlimit,pid,iid,sid,throttle,lbtot,tracked,type,rate,rate_lim,rate_max,check_status,check_code,check_duration,hrsp_1xx,hrsp_2xx,hrsp_3xx,hrsp_4xx,hrsp_5xx,hrsp_other,hanafail,req_rate,req_rate_max,req_tot,cli_abrt,srv_abrt,comp_in,comp_out,comp_byp,comp_rsp,lastsess,
	expectedCsvFieldCount = 52
	statusField           = 17
)

var (
	frontendLabelNames = []string{"frontend"}
	backendLabelNames  = []string{"backend"}
	serverLabelNames   = []string{"backend", "server"}
)

func newFrontendMetric(metricName string, docString string, constLabels prometheus.Labels) *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   namespace,
			Name:        "frontend_" + metricName,
			Help:        docString,
			ConstLabels: constLabels,
		},
		frontendLabelNames,
	)
}

func newBackendMetric(metricName string, docString string, constLabels prometheus.Labels) *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   namespace,
			Name:        "backend_" + metricName,
			Help:        docString,
			ConstLabels: constLabels,
		},
		backendLabelNames,
	)
}

func newServerMetric(metricName string, docString string, constLabels prometheus.Labels) *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   namespace,
			Name:        "server_" + metricName,
			Help:        docString,
			ConstLabels: constLabels,
		},
		serverLabelNames,
	)
}

type metrics map[int]*prometheus.GaugeVec

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
		2:  newServerMetric("current_queue", "Current number of queued requests assigned to this server.", prometheus.Labels{"uri": ""}),
		3:  newServerMetric("max_queue", "Maximum observed number of queued requests assigned to this server.", prometheus.Labels{"uri": ""}),
		4:  newServerMetric("current_sessions", "Current number of active sessions.", prometheus.Labels{"uri": ""}),
		5:  newServerMetric("max_sessions", "Maximum observed number of active sessions.", prometheus.Labels{"uri": ""}),
		7:  newServerMetric("connections_total", "Total number of connections.", prometheus.Labels{"uri": ""}),
		8:  newServerMetric("bytes_in_total", "Current total of incoming bytes.", prometheus.Labels{"uri": ""}),
		9:  newServerMetric("bytes_out_total", "Current total of outgoing bytes.", prometheus.Labels{"uri": ""}),
		13: newServerMetric("connection_errors_total", "Total of connection errors.", prometheus.Labels{"uri": ""}),
		14: newServerMetric("response_errors_total", "Total of response errors.", prometheus.Labels{"uri": ""}),
		15: newServerMetric("retry_warnings_total", "Total of retry warnings.", prometheus.Labels{"uri": ""}),
		16: newServerMetric("redispatch_warnings_total", "Total of redispatch warnings.", prometheus.Labels{"uri": ""}),
		17: newServerMetric("up", "Current health status of the server (1 = UP, 0 = DOWN).", prometheus.Labels{"uri": ""}),
		18: newServerMetric("weight", "Current weight of the server.", prometheus.Labels{"uri": ""}),
		21: newServerMetric("check_failures_total", "Total number of failed health checks.", prometheus.Labels{"uri": ""}),
		24: newServerMetric("downtime_seconds_total", "Total downtime in seconds.", prometheus.Labels{"uri": ""}),
		33: newServerMetric("current_session_rate", "Current number of sessions per second over last elapsed second.", prometheus.Labels{"uri": ""}),
		35: newServerMetric("max_session_rate", "Maximum observed number of sessions per second.", prometheus.Labels{"uri": ""}),
		38: newServerMetric("check_duration_milliseconds", "Previously run health check duration, in milliseconds", prometheus.Labels{"uri": ""}),
		39: newServerMetric("http_responses_total", "Total of HTTP responses.", prometheus.Labels{"code": "1xx", "uri": ""}),
		40: newServerMetric("http_responses_total", "Total of HTTP responses.", prometheus.Labels{"code": "2xx", "uri": ""}),
		41: newServerMetric("http_responses_total", "Total of HTTP responses.", prometheus.Labels{"code": "3xx", "uri": ""}),
		42: newServerMetric("http_responses_total", "Total of HTTP responses.", prometheus.Labels{"code": "4xx", "uri": ""}),
		43: newServerMetric("http_responses_total", "Total of HTTP responses.", prometheus.Labels{"code": "5xx", "uri": ""}),
		44: newServerMetric("http_responses_total", "Total of HTTP responses.", prometheus.Labels{"code": "other", "uri": ""}),
	}
)

// Exporter collects HAProxy stats from the given URI and exports them using
// the prometheus metrics package.
type Exporter struct {
	URI   string
	mutex sync.RWMutex
	fetch func() (io.ReadCloser, error)

	up                                             prometheus.Gauge
	totalScrapes, csvParseFailures                 prometheus.Counter
	frontendMetrics, backendMetrics, serverMetrics map[int]*prometheus.GaugeVec
}

// NewExporter returns an initialized Exporter.
func NewExporter(uri string, selectedServerMetrics map[int]*prometheus.GaugeVec, timeout time.Duration) (*Exporter, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}

	var fetch func() (io.ReadCloser, error)
	switch u.Scheme {
	case "http", "https", "file":
		fetch = fetchHTTP(uri, timeout)
	case "unix":
		fetch = fetchUnix(u, timeout)
	default:
		return nil, fmt.Errorf("unsupported scheme: %q", u.Scheme)
	}
	uriLabel := prometheus.Labels{"uri": uri}
	return &Exporter{
		URI:   uri,
		fetch: fetch,
		up: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace:   namespace,
			Name:        "up",
			Help:        "Was the last scrape of haproxy successful.",
			ConstLabels: uriLabel,
		}),
		totalScrapes: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace:   namespace,
			Name:        "exporter_total_scrapes",
			Help:        "Current total HAProxy scrapes.",
			ConstLabels: uriLabel,
		}),
		csvParseFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace:   namespace,
			Name:        "exporter_csv_parse_failures",
			Help:        "Number of errors while parsing CSV.",
			ConstLabels: uriLabel,
		}),
		frontendMetrics: map[int]*prometheus.GaugeVec{
			4:  newFrontendMetric("current_sessions", "Current number of active sessions.", uriLabel),
			5:  newFrontendMetric("max_sessions", "Maximum observed number of active sessions.", uriLabel),
			6:  newFrontendMetric("limit_sessions", "Configured session limit.", uriLabel),
			7:  newFrontendMetric("connections_total", "Total number of connections.", uriLabel),
			8:  newFrontendMetric("bytes_in_total", "Current total of incoming bytes.", uriLabel),
			9:  newFrontendMetric("bytes_out_total", "Current total of outgoing bytes.", uriLabel),
			10: newFrontendMetric("requests_denied_total", "Total of requests denied for security.", uriLabel),
			12: newFrontendMetric("request_errors_total", "Total of request errors.", uriLabel),
			33: newFrontendMetric("current_session_rate", "Current number of sessions per second over last elapsed second.", uriLabel),
			34: newFrontendMetric("limit_session_rate", "Configured limit on new sessions per second.", uriLabel),
			35: newFrontendMetric("max_session_rate", "Maximum observed number of sessions per second.", uriLabel),
			39: newFrontendMetric("http_responses_total", "Total of HTTP responses.", prometheus.Labels{"code": "1xx", "uri": uri}),
			40: newFrontendMetric("http_responses_total", "Total of HTTP responses.", prometheus.Labels{"code": "2xx", "uri": uri}),
			41: newFrontendMetric("http_responses_total", "Total of HTTP responses.", prometheus.Labels{"code": "3xx", "uri": uri}),
			42: newFrontendMetric("http_responses_total", "Total of HTTP responses.", prometheus.Labels{"code": "4xx", "uri": uri}),
			43: newFrontendMetric("http_responses_total", "Total of HTTP responses.", prometheus.Labels{"code": "5xx", "uri": uri}),
			44: newFrontendMetric("http_responses_total", "Total of HTTP responses.", prometheus.Labels{"code": "other", "uri": uri}),
			48: newFrontendMetric("http_requests_total", "Total HTTP requests.", uriLabel),
		},
		backendMetrics: map[int]*prometheus.GaugeVec{
			2:  newBackendMetric("current_queue", "Current number of queued requests not assigned to any server.", uriLabel),
			3:  newBackendMetric("max_queue", "Maximum observed number of queued requests not assigned to any server.", uriLabel),
			4:  newBackendMetric("current_sessions", "Current number of active sessions.", uriLabel),
			5:  newBackendMetric("max_sessions", "Maximum observed number of active sessions.", uriLabel),
			6:  newBackendMetric("limit_sessions", "Configured session limit.", uriLabel),
			7:  newBackendMetric("connections_total", "Total number of connections.", uriLabel),
			8:  newBackendMetric("bytes_in_total", "Current total of incoming bytes.", uriLabel),
			9:  newBackendMetric("bytes_out_total", "Current total of outgoing bytes.", uriLabel),
			13: newBackendMetric("connection_errors_total", "Total of connection errors.", uriLabel),
			14: newBackendMetric("response_errors_total", "Total of response errors.", uriLabel),
			15: newBackendMetric("retry_warnings_total", "Total of retry warnings.", uriLabel),
			16: newBackendMetric("redispatch_warnings_total", "Total of redispatch warnings.", uriLabel),
			17: newBackendMetric("up", "Current health status of the backend (1 = UP, 0 = DOWN).", uriLabel),
			18: newBackendMetric("weight", "Total weight of the servers in the backend.", uriLabel),
			33: newBackendMetric("current_session_rate", "Current number of sessions per second over last elapsed second.", uriLabel),
			35: newBackendMetric("max_session_rate", "Maximum number of sessions per second.", uriLabel),
			39: newBackendMetric("http_responses_total", "Total of HTTP responses.", prometheus.Labels{"code": "1xx", "uri": uri}),
			40: newBackendMetric("http_responses_total", "Total of HTTP responses.", prometheus.Labels{"code": "2xx", "uri": uri}),
			41: newBackendMetric("http_responses_total", "Total of HTTP responses.", prometheus.Labels{"code": "3xx", "uri": uri}),
			42: newBackendMetric("http_responses_total", "Total of HTTP responses.", prometheus.Labels{"code": "4xx", "uri": uri}),
			43: newBackendMetric("http_responses_total", "Total of HTTP responses.", prometheus.Labels{"code": "5xx", "uri": uri}),
			44: newBackendMetric("http_responses_total", "Total of HTTP responses.", prometheus.Labels{"code": "other", "uri": uri}),
		},
		serverMetrics: selectedServerMetrics,
	}, nil
}

// Describe describes all the metrics ever exported by the HAProxy exporter. It
// implements prometheus.Collector.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	for _, m := range e.frontendMetrics {
		m.Describe(ch)
	}
	for _, m := range e.backendMetrics {
		m.Describe(ch)
	}
	for _, m := range e.serverMetrics {
		m.Describe(ch)
	}
	ch <- e.up.Desc()
	ch <- e.totalScrapes.Desc()
	ch <- e.csvParseFailures.Desc()
}

// Collect fetches the stats from configured HAProxy location and delivers them
// as Prometheus metrics. It implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.mutex.Lock() // To protect metrics from concurrent collects.
	defer e.mutex.Unlock()

	e.resetMetrics()
	e.scrape()

	ch <- e.up
	ch <- e.totalScrapes
	ch <- e.csvParseFailures
	e.collectMetrics(ch)
}

func fetchHTTP(uri string, timeout time.Duration) func() (io.ReadCloser, error) {
	client := http.Client{
		Timeout: timeout,
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

func (e *Exporter) scrape() {
	e.totalScrapes.Inc()

	body, err := e.fetch()
	if err != nil {
		e.up.Set(0)
		log.Errorf("Can't scrape HAProxy: %v", err)
		return
	}
	defer body.Close()
	e.up.Set(1)

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
				log.Errorf("Can't read CSV: %v", err)
				e.csvParseFailures.Inc()
				continue loop
			}
			log.Errorf("Unexpected error while reading CSV: %v", err)
			e.up.Set(0)
			break loop
		}
		e.parseRow(row)
	}
}

func (e *Exporter) resetMetrics() {
	for _, m := range e.frontendMetrics {
		m.Reset()
	}
	for _, m := range e.backendMetrics {
		m.Reset()
	}
	for _, m := range e.serverMetrics {
		m.Reset()
	}
}

func (e *Exporter) collectMetrics(metrics chan<- prometheus.Metric) {
	for _, m := range e.frontendMetrics {
		m.Collect(metrics)
	}
	for _, m := range e.backendMetrics {
		m.Collect(metrics)
	}
	for _, m := range e.serverMetrics {
		m.Collect(metrics)
	}
}

func (e *Exporter) parseRow(csvRow []string) {
	if len(csvRow) < expectedCsvFieldCount {
		log.Errorf("Wrong CSV field count: %d vs. %d", len(csvRow), expectedCsvFieldCount)
		e.csvParseFailures.Inc()
		return
	}

	pxname, svname, type_ := csvRow[0], csvRow[1], csvRow[32]

	const (
		frontend = "0"
		backend  = "1"
		server   = "2"
		listener = "3"
	)

	switch type_ {
	case frontend:
		e.exportCsvFields(e.frontendMetrics, csvRow, pxname)
	case backend:
		e.exportCsvFields(e.backendMetrics, csvRow, pxname)
	case server:
		e.exportCsvFields(e.serverMetrics, csvRow, pxname, svname)
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

func (e *Exporter) exportCsvFields(metrics map[int]*prometheus.GaugeVec, csvRow []string, labels ...string) {
	for fieldIdx, metric := range metrics {
		valueStr := csvRow[fieldIdx]
		if valueStr == "" {
			continue
		}

		var value int64
		switch fieldIdx {
		case statusField:
			value = parseStatusField(valueStr)
		default:
			var err error
			value, err = strconv.ParseInt(valueStr, 10, 64)
			if err != nil {
				log.Errorf("Can't parse CSV field value %s: %v", valueStr, err)
				e.csvParseFailures.Inc()
				continue
			}
		}
		metric.WithLabelValues(labels...).Set(float64(value))
	}
}

// filterServerMetrics returns the set of server metrics specified by the comma
// separated filter.
func filterServerMetrics(filter, uri string) (map[int]*prometheus.GaugeVec, error) {
	selectedMetrics := map[int]*prometheus.GaugeVec{}
	if len(filter) == 0 {
		return selectedMetrics, nil
	}

	selected := map[int]struct{}{}
	for _, f := range strings.Split(filter, ",") {
		field, err := strconv.Atoi(f)
		if err != nil {
			return nil, fmt.Errorf("invalid server metric field number: %v", f)
		}
		selected[field] = struct{}{}
	}

	socketLabel := prometheus.Labels{"uri": uri}
	var serverMetrics = metrics{
		2:  newServerMetric("current_queue", "Current number of queued requests assigned to this server.", socketLabel),
		3:  newServerMetric("max_queue", "Maximum observed number of queued requests assigned to this server.", socketLabel),
		4:  newServerMetric("current_sessions", "Current number of active sessions.", socketLabel),
		5:  newServerMetric("max_sessions", "Maximum observed number of active sessions.", socketLabel),
		7:  newServerMetric("connections_total", "Total number of connections.", socketLabel),
		8:  newServerMetric("bytes_in_total", "Current total of incoming bytes.", socketLabel),
		9:  newServerMetric("bytes_out_total", "Current total of outgoing bytes.", socketLabel),
		13: newServerMetric("connection_errors_total", "Total of connection errors.", socketLabel),
		14: newServerMetric("response_errors_total", "Total of response errors.", socketLabel),
		15: newServerMetric("retry_warnings_total", "Total of retry warnings.", socketLabel),
		16: newServerMetric("redispatch_warnings_total", "Total of redispatch warnings.", socketLabel),
		17: newServerMetric("up", "Current health status of the server (1 = UP, 0 = DOWN).", socketLabel),
		18: newServerMetric("weight", "Current weight of the server.", socketLabel),
		21: newServerMetric("check_failures_total", "Total number of failed health checks.", socketLabel),
		24: newServerMetric("downtime_seconds_total", "Total downtime in seconds.", socketLabel),
		33: newServerMetric("current_session_rate", "Current number of sessions per second over last elapsed second.", socketLabel),
		35: newServerMetric("max_session_rate", "Maximum observed number of sessions per second.", socketLabel),
		38: newServerMetric("check_duration_milliseconds", "Previously run health check duration, in milliseconds", socketLabel),
		39: newServerMetric("http_responses_total", "Total of HTTP responses.", prometheus.Labels{"code": "1xx", "uri": uri}),
		40: newServerMetric("http_responses_total", "Total of HTTP responses.", prometheus.Labels{"code": "2xx", "uri": uri}),
		41: newServerMetric("http_responses_total", "Total of HTTP responses.", prometheus.Labels{"code": "3xx", "uri": uri}),
		42: newServerMetric("http_responses_total", "Total of HTTP responses.", prometheus.Labels{"code": "4xx", "uri": uri}),
		43: newServerMetric("http_responses_total", "Total of HTTP responses.", prometheus.Labels{"code": "5xx", "uri": uri}),
		44: newServerMetric("http_responses_total", "Total of HTTP responses.", prometheus.Labels{"code": "other", "uri": uri}),
	}

	for field, metric := range serverMetrics {
		if _, ok := selected[field]; ok {
			selectedMetrics[field] = metric
		}
	}
	return selectedMetrics, nil
}

func main() {
	var (
		listenAddress             = flag.String("web.listen-address", ":9101", "Address to listen on for web interface and telemetry.")
		metricsPath               = flag.String("web.telemetry-path", "/metrics", "Path under which to expose metrics.")
		haProxyScrapeURI          = flag.String("haproxy.scrape-uri", "http://localhost/;csv", "URI on which to scrape HAProxy.")
		haProxyServerMetricFields = flag.String("haproxy.server-metric-fields", serverMetrics.String(), "Comma-seperated list of exported server metrics. See http://cbonte.github.io/haproxy-dconv/configuration-1.5.html#9.1")
		haProxyTimeout            = flag.Duration("haproxy.timeout", 5*time.Second, "Timeout for trying to get stats from HAProxy.")
		haProxyPidFile            = flag.String("haproxy.pid-file", "", "Path to haproxy's pid file.")
		showVersion               = flag.Bool("version", false, "Print version information.")
	)
	flag.Parse()

	if *showVersion {
		fmt.Fprintln(os.Stdout, version.Print("haproxy_exporter"))
		os.Exit(0)
	}

	log.Infoln("Starting haproxy_exporter", version.Info())
	log.Infoln("Build context", version.BuildContext())

	u, err := url.Parse(*haProxyScrapeURI)
	if err != nil {
		log.Fatal(err)
	}
	var matches []string
	if u.Scheme == "unix" {
		var err error
		matches, err = filepath.Glob(u.Path)
		if err != nil {
			log.Fatal(err)
		}
		for i := range matches {
			matches[i] = "unix:" + matches[i]
		}
	} else {
		matches = []string{*haProxyScrapeURI}
	}
	for _, match := range matches {
		selectedServerMetrics, err := filterServerMetrics(*haProxyServerMetricFields, match)
		if err != nil {
			log.Fatal(err)
		}

		exporter, err := NewExporter(match, selectedServerMetrics, *haProxyTimeout)
		if err != nil {
			log.Fatal(err)
		}
		prometheus.MustRegister(exporter)
	}

	prometheus.MustRegister(version.NewCollector("haproxy_exporter"))

	if *haProxyPidFile != "" {
		procExporter := prometheus.NewProcessCollectorPIDFn(
			func() (int, error) {
				content, err := ioutil.ReadFile(*haProxyPidFile)
				if err != nil {
					return 0, fmt.Errorf("Can't read pid file: %s", err)
				}
				value, err := strconv.Atoi(strings.TrimSpace(string(content)))
				if err != nil {
					return 0, fmt.Errorf("Can't parse pid file: %s", err)
				}
				return value, nil
			}, namespace)
		prometheus.MustRegister(procExporter)
	}

	log.Infoln("Listening on", *listenAddress)
	http.Handle(*metricsPath, prometheus.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
             <head><title>Haproxy Exporter</title></head>
             <body>
             <h1>Haproxy Exporter</h1>
             <p><a href='` + *metricsPath + `'>Metrics</a></p>
             </body>
             </html>`))
	})
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}
