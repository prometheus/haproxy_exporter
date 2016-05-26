package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
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

type metricsMap map[int]*MetricVecProxy

func newMetricsMap(prefix string, labelNames []string, idxs []int) metricsMap {
	m := make(metricsMap, len(idxs))

	for _, v := range idxs {
		spec := metricSpecs[v]
		m[v] = NewMetricVecProxy(
			spec.metricFamily,
			prometheus.Opts{
				Namespace:   namespace,
				Name:        prefix + "_" + spec.metricName,
				Help:        spec.docString,
				ConstLabels: spec.metricLabels,
			},
			labelNames,
		)
	}

	return m
}

func (m metricsMap) String() string {
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
	serverMetrics = newMetricsMap(
		"server",
		serverLabelNames,
		[]int{2, 3, 4, 5, 7, 8, 9, 13, 14, 15, 16, 17, 18, 21, 24, 33, 35, 38, 39, 40, 41, 42, 43, 44},
	)
)

// Exporter collects HAProxy stats from the given URI and exports them using
// the prometheus metrics package.
type Exporter struct {
	URI   string
	mutex sync.RWMutex

	up                                             prometheus.Gauge
	totalScrapes, csvParseFailures                 prometheus.Counter
	frontendMetrics, backendMetrics, serverMetrics metricsMap
	client                                         *http.Client
}

// NewExporter returns an initialized Exporter.
func NewExporter(uri string, selectedServerMetrics metricsMap, timeout time.Duration) *Exporter {
	return &Exporter{
		URI: uri,
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
		frontendMetrics: newMetricsMap(
			"frontend",
			frontendLabelNames,
			[]int{4, 5, 6, 7, 8, 9, 10, 12, 33, 34, 35, 39, 40, 41, 42, 43, 44, 48},
		),
		backendMetrics: newMetricsMap(
			"backend",
			backendLabelNames,
			[]int{2, 3, 4, 5, 6, 7, 8, 9, 13, 14, 15, 16, 17, 18, 33, 35, 39, 40, 41, 42, 43, 44},
		),
		serverMetrics: selectedServerMetrics,
		client: &http.Client{
			Transport: &http.Transport{
				Dial: func(netw, addr string) (net.Conn, error) {
					c, err := net.DialTimeout(netw, addr, timeout)
					if err != nil {
						return nil, err
					}
					if err := c.SetDeadline(time.Now().Add(timeout)); err != nil {
						return nil, err
					}
					return c, nil
				},
			},
		},
	}
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

func (e *Exporter) scrape() {
	e.totalScrapes.Inc()

	resp, err := e.client.Get(e.URI)
	if err != nil {
		e.up.Set(0)
		log.Errorf("Can't scrape HAProxy: %v", err)
		return
	}
	defer resp.Body.Close()
	if !(resp.StatusCode >= 200 && resp.StatusCode < 300) {
		e.up.Set(0)
		log.Errorf("Can't scrape HAProxy: status %d", resp.StatusCode)
		return
	}
	e.up.Set(1)

	reader := csv.NewReader(resp.Body)
	reader.TrailingComma = true
	reader.Comment = '#'

loop:
	for {
		row, err := reader.Read()
		switch err {
		case nil:
		case io.EOF:
			break loop
		case err.(*csv.ParseError):
			log.Errorf("Can't read CSV: %v", err)
			e.csvParseFailures.Inc()
			continue loop
		default:
			log.Errorf("Unexpected error while reading CSV: %v", err)
			e.csvParseFailures.Inc()
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

func (e *Exporter) exportCsvFields(metrics metricsMap, csvRow []string, labels ...string) {
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
func filterServerMetrics(filter string) (metricsMap, error) {
	metrics := make(metricsMap)

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

	selectedServerMetrics, err := filterServerMetrics(*haProxyServerMetricFields)
	if err != nil {
		log.Fatal(err)
	}

	log.Infoln("Starting haproxy_exporter", version.Info())
	log.Infoln("Build context", version.BuildContext())

	exporter := NewExporter(*haProxyScrapeURI, selectedServerMetrics, *haProxyTimeout)
	prometheus.MustRegister(exporter)
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
