package main

import (
	"encoding/csv"
	"flag"
	"io"
	"log"
	"net/http"
	_ "net/http/pprof"
	"strconv"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	namespace = "haproxy" // For Prometheus metrics.

	// HAProxy 1.4
	// # pxname,svname,qcur,qmax,scur,smax,slim,stot,bin,bout,dreq,dresp,ereq,econ,eresp,wretr,wredis,status,weight,act,bck,chkfail,chkdown,lastchg,downtime,qlimit,pid,iid,sid,throttle,lbtot,tracked,type,rate,rate_lim,rate_max,check_status,check_code,check_duration,hrsp_1xx,hrsp_2xx,hrsp_3xx,hrsp_4xx,hrsp_5xx,hrsp_other,hanafail,req_rate,req_rate_max,req_tot,cli_abrt,srv_abrt,
	// HAProxy 1.5
	// pxname,svname,qcur,qmax,scur,smax,slim,stot,bin,bout,dreq,dresp,ereq,econ,eresp,wretr,wredis,status,weight,act,bck,chkfail,chkdown,lastchg,downtime,qlimit,pid,iid,sid,throttle,lbtot,tracked,type,rate,rate_lim,rate_max,check_status,check_code,check_duration,hrsp_1xx,hrsp_2xx,hrsp_3xx,hrsp_4xx,hrsp_5xx,hrsp_other,hanafail,req_rate,req_rate_max,req_tot,cli_abrt,srv_abrt,comp_in,comp_out,comp_byp,comp_rsp,lastsess,
	expectedCsvFieldCount = 52
)

var (
	serviceLabelNames = []string{"service"}
	backendLabelNames = []string{"service", "server"}
)

func newServiceMetric(metricName string, docString string) *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      metricName,
			Help:      docString,
		},
		serviceLabelNames,
	)
}

func newBackendMetric(metricName string, docString string, constLabels prometheus.Labels) *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   namespace,
			Name:        metricName,
			Help:        docString,
			ConstLabels: constLabels,
		},
		backendLabelNames,
	)
}

// Exporter collects HAProxy stats from the given URI and exports them using
// the prometheus metrics package.
type Exporter struct {
	URI   string
	mutex sync.RWMutex

	totalScrapes, scrapeFailures, csvParseFailures prometheus.Counter
	serviceMetrics, backendMetrics                 map[int]*prometheus.GaugeVec
}

// NewExporter returns an initialized Exporter.
func NewExporter(uri string) *Exporter {
	return &Exporter{
		URI: uri,
		totalScrapes: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "exporter_total_scrapes",
			Help:      "Current total HAProxy scrapes.",
		}),
		scrapeFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "exporter_scrape_failures",
			Help:      "Number of errors while scraping HAProxy.",
		}),
		csvParseFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "exporter_csv_parse_failures",
			Help:      "Number of errors while parsing CSV.",
		}),
		serviceMetrics: map[int]*prometheus.GaugeVec{
			2: newServiceMetric("current_queue", "Current server queue length."),
			3: newServiceMetric("max_queue", "Maximum server queue length."),
		},
		backendMetrics: map[int]*prometheus.GaugeVec{
			4:  newBackendMetric("current_sessions", "Current number of active sessions.", nil),
			5:  newBackendMetric("max_sessions", "Maximum number of active sessions.", nil),
			8:  newBackendMetric("bytes_in", "Current total of incoming bytes.", nil),
			9:  newBackendMetric("bytes_out", "Current total of outgoing bytes.", nil),
			13: newBackendMetric("connection_errors", "Total of connection errors.", nil),
			14: newBackendMetric("response_errors", "Total of response errors.", nil),
			15: newBackendMetric("retry_warnings", "Total of retry warnings.", nil),
			16: newBackendMetric("redispatch_warnings", "Total of redispatch warnings.", nil),
			17: newBackendMetric("server_up", "Current health status of the server (1 = UP, 0 = DOWN).", nil),
			33: newBackendMetric("current_session_rate", "Current number of sessions per second over last elapsed second.", nil),
			35: newBackendMetric("max_session_rate", "Maximum number of sessions per second.", nil),
			40: newBackendMetric("http_responses", "Total of HTTP responses.", prometheus.Labels{"code": "2xx"}),
			41: newBackendMetric("http_responses", "Total of HTTP responses.", prometheus.Labels{"code": "3xx"}),
			42: newBackendMetric("http_responses", "Total of HTTP responses.", prometheus.Labels{"code": "4xx"}),
			43: newBackendMetric("http_responses", "Total of HTTP responses.", prometheus.Labels{"code": "5xx"}),
		},
	}
}

// Describe describes all the metrics ever exported by the HAProxy exporter. It
// implements prometheus.Collector.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	for _, m := range e.serviceMetrics {
		m.Describe(ch)
	}
	for _, m := range e.backendMetrics {
		m.Describe(ch)
	}
	ch <- e.totalScrapes.Desc()
	ch <- e.scrapeFailures.Desc()
	ch <- e.csvParseFailures.Desc()
}

// Collect fetches the stats from configured HAProxy location and delivers them
// as Prometheus metrics. It implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	csvRows := make(chan []string)

	go e.scrape(csvRows)

	e.mutex.Lock() // To protect metrics from concurrent collects.
	defer e.mutex.Unlock()
	e.resetMetrics()
	e.setMetrics(csvRows)
	ch <- e.totalScrapes
	ch <- e.scrapeFailures
	ch <- e.csvParseFailures
	e.collectMetrics(ch)
}

func (e *Exporter) scrape(csvRows chan<- []string) {
	defer close(csvRows)

	e.totalScrapes.Inc()

	resp, err := http.Get(e.URI)
	if err != nil {
		log.Printf("Error while scraping HAProxy: %v", err)
		e.scrapeFailures.Inc()
		return
	}
	defer resp.Body.Close()

	reader := csv.NewReader(resp.Body)
	reader.TrailingComma = true
	reader.Comment = '#'

	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("Error while reading CSV: %v", err)
			e.csvParseFailures.Inc()
			break
		}
		if len(row) == 0 {
			continue
		}
		csvRows <- row
	}
}

func (e *Exporter) resetMetrics() {
	for _, m := range e.serviceMetrics {
		m.Reset()
	}
	for _, m := range e.backendMetrics {
		m.Reset()
	}
}

func (e *Exporter) collectMetrics(metrics chan<- prometheus.Metric) {
	for _, m := range e.serviceMetrics {
		m.Collect(metrics)
	}
	for _, m := range e.backendMetrics {
		m.Collect(metrics)
	}
}

func (e *Exporter) setMetrics(csvRows <-chan []string) {
	for csvRow := range csvRows {
		if len(csvRow) < expectedCsvFieldCount {
			log.Printf("Wrong CSV field count: %d vs. %d", len(csvRow), expectedCsvFieldCount)
			e.csvParseFailures.Inc()
			continue
		}

		service, server := csvRow[0], csvRow[1]

		if server == "FRONTEND" {
			continue
		}

		if server == "BACKEND" {
			e.exportCsvFields(e.serviceMetrics, csvRow, service)
		} else {
			e.exportCsvFields(e.backendMetrics, csvRow, service, server)
		}
	}
}

func (e *Exporter) exportCsvFields(metrics map[int]*prometheus.GaugeVec, csvRow []string, labels ...string) {
	for fieldIdx, metric := range metrics {
		valueStr := csvRow[fieldIdx]
		if valueStr == "" {
			continue
		}

		var value int64
		var err error
		switch valueStr {
		// UP or UP going down
		case "UP", "UP 1/3", "UP 2/3":
			value = 1
		// DOWN or DOWN going up
		case "DOWN", "DOWN 1/2":
			value = 0
		case "OPEN":
			value = 0
		case "no check":
			continue
		default:
			value, err = strconv.ParseInt(valueStr, 10, 64)
			if err != nil {
				log.Printf("Error while parsing CSV field value %s: %v", valueStr, err)
				e.csvParseFailures.Inc()
				continue
			}
		}
		metric.WithLabelValues(labels...).Set(float64(value))
	}
}

func main() {
	var (
		listeningAddress = flag.String("telemetry.address", ":8080", "Address on which to expose metrics.")
		metricsEndpoint  = flag.String("telemetry.endpoint", "/metrics", "Path under which to expose metrics.")
		haProxyScrapeUri = flag.String("haproxy.scrape_uri", "http://localhost/;csv", "URI on which to scrape HAProxy.")
		_                = flag.Duration("haproxy.scrape_interval", 0, "DEPRECATED. Not used anymore.")
	)
	flag.Parse()

	exporter := NewExporter(*haProxyScrapeUri)
	prometheus.MustRegister(exporter)

	log.Printf("Starting Server: %s", *listeningAddress)
	http.Handle(*metricsEndpoint, prometheus.Handler())
	log.Fatal(http.ListenAndServe(*listeningAddress, nil))
}
