package main

import (
	"encoding/csv"
	"flag"
	"io"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const expectedCsvFieldCount = 52

var (
	totalScrapes     = prometheus.NewCounter()
	scrapeFailures   = prometheus.NewCounter()
	csvParseFailures = prometheus.NewCounter()
)

type registry struct {
	prometheus.Registry
	serviceMetrics map[int]prometheus.Gauge
	backendMetrics map[int]prometheus.Gauge
}

func newRegistry() *registry {
	r := &registry{prometheus.NewRegistry(), make(map[int]prometheus.Gauge), make(map[int]prometheus.Gauge)}

	r.Register("haproxy_exporter_total_scrapes", "Current total HAProxy scrapes.", prometheus.NilLabels, totalScrapes)
	r.Register("haproxy_exporter_scrape_failures", "Number of errors while scraping HAProxy.", prometheus.NilLabels, scrapeFailures)
	r.Register("haproxy_exporter_csv_parse_failures", "Number of errors while parsing CSV.", prometheus.NilLabels, csvParseFailures)

	r.serviceMetrics = map[int]prometheus.Gauge{
		2: r.newGauge("haproxy_current_queue", "Current server queue length."),
		3: r.newGauge("haproxy_max_queue", "Maximum server queue length."),
	}

	r.backendMetrics = map[int]prometheus.Gauge{
		4:  r.newGauge("haproxy_current_sessions", "Current number of active sessions."),
		5:  r.newGauge("haproxy_max_sessions", "Maximum number of active sessions."),
		8:  r.newGauge("haproxy_bytes_in", "Current total of incoming bytes."),
		9:  r.newGauge("haproxy_bytes_out", "Current total of outgoing bytes."),
		17: r.newGauge("haproxy_instance_up", "Current health status of the instance (1 = UP, 0 = DOWN)."),
		33: r.newGauge("haproxy_current_session_rate", "Current number of sessions per second."),
		35: r.newGauge("haproxy_max_session_rate", "Maximum number of sessions per second."),
	}

	return r
}

func (r *registry) newGauge(metricName string, docString string) prometheus.Gauge {
	gauge := prometheus.NewGauge()
	r.Register(metricName, docString, prometheus.NilLabels, gauge)
	return gauge
}

// Exporter collects HAProxy stats from the given URI and exports them using
// the prometheus metrics package.
type Exporter struct {
	URI   string
	reg   *registry
	mutex sync.RWMutex
}

// NewExporter returns an initialized Exporter.
func NewExporter(uri string) *Exporter {
	return &Exporter{
		URI: uri,
		reg: newRegistry(),
	}
}

// Registry returns a prometheus.Registry type with the complete state of the
// last stats collection run.
func (e *Exporter) Registry() prometheus.Registry {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	return e.reg
}

// Handler returns a http.HandlerFunc of the last finished prometheus.Registry.
func (e *Exporter) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		reg := e.Registry()
		f := reg.Handler()

		f(w, r)
	}
}

// Scrape fetches the stats from configured HAProxy location. It creates a new
// prometheus.Registry object every time to not leak stale data from previous
// collections.
func (e *Exporter) Scrape() {
	csvRows := make(chan []string)
	quitChan := make(chan bool)
	reg := newRegistry()

	go e.scrape(csvRows, quitChan)
	reg.exportMetrics(csvRows, quitChan)

	e.mutex.Lock()
	defer e.mutex.Unlock()

	e.reg = reg
}

// ScrapePeriodically runs the Scrape function in the specified interval.
func (e *Exporter) ScrapePeriodically(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for _ = range ticker.C {
		e.Scrape()
	}
}

func (e *Exporter) scrape(csvRows chan []string, quitChan chan bool) {
	defer close(quitChan)
	defer totalScrapes.Increment(prometheus.NilLabels)

	resp, err := http.Get(e.URI)
	if err != nil {
		log.Printf("Error while scraping HAProxy: %v", err)
		scrapeFailures.Increment(prometheus.NilLabels)
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
			csvParseFailures.Increment(prometheus.NilLabels)
			return
		}
		if len(row) == 0 {
			continue
		}

		csvRows <- row
	}

}

func (r *registry) exportMetrics(csvRows chan []string, quitChan chan bool) {
	for {
		select {
		case row := <-csvRows:
			r.exportCsvRow(row)
		case <-quitChan:
			return
		}
	}
}

func (r *registry) exportCsvRow(csvRow []string) {
	if len(csvRow) != expectedCsvFieldCount {
		log.Printf("Wrong CSV field count: %d vs. %d", len(csvRow), expectedCsvFieldCount)
		csvParseFailures.Increment(prometheus.NilLabels)
		return
	}

	service, instance := csvRow[0], csvRow[1]

	if instance == "FRONTEND" {
		return
	}

	if instance == "BACKEND" {
		labels := map[string]string{
			"service": service,
		}

		exportCsvFields(labels, r.serviceMetrics, csvRow)
	} else {
		labels := map[string]string{
			"service":  service,
			"instance": instance,
		}

		exportCsvFields(labels, r.backendMetrics, csvRow)
	}
}

func exportCsvFields(labels map[string]string, fields map[int]prometheus.Gauge, csvRow []string) {
	for fieldIdx, gauge := range fields {
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
				csvParseFailures.Increment(prometheus.NilLabels)
				continue
			}
		}
		gauge.Set(labels, float64(value))
	}
}

func main() {
	var (
		listeningAddress      = flag.String("telemetry.address", ":8080", "Address on which to expose JSON metrics.")
		metricsEndpoint       = flag.String("telemetry.endpoint", prometheus.ExpositionResource, "Path under which to expose metrics.")
		haProxyScrapeUri      = flag.String("haproxy.scrape_uri", "http://localhost/;csv", "URI on which to scrape HAProxy.")
		haProxyScrapeInterval = flag.Duration("haproxy.scrape_interval", 15*time.Second, "Interval in seconds between scrapes.")
	)
	flag.Parse()

	exporter := NewExporter(*haProxyScrapeUri)
	go exporter.ScrapePeriodically(*haProxyScrapeInterval)

	log.Printf("Starting Server: %s", *listeningAddress)
	http.Handle(*metricsEndpoint, exporter.Handler())
	log.Fatal(http.ListenAndServe(*listeningAddress, nil))
}
