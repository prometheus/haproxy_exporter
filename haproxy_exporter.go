package main

import (
	"encoding/csv"
	"flag"
	"github.com/matttproud/golang_instrumentation"
	"github.com/matttproud/golang_instrumentation/metrics"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"
)

// Constants.
const expectedCsvFieldCount = 52

// Commandline flags.
var (
	listeningAddress      = flag.String("listeningAddress", ":8080", "Address on which to expose JSON metrics.")
	metricsEndpoint       = flag.String("metricsEndpoint", "/metrics.json", "Path under which to expose JSON metrics.")
	haProxyScrapeUri      = flag.String("haProxyScrapeUri", "http://localhost/;csv", "URI on which to scrape HAProxy.")
	haProxyScrapeInterval = flag.Duration("haProxyScrapeInterval", 15, "Interval in seconds between scrapes.")
)

// Exported internal metrics.
var (
	totalScrapes     = metrics.NewCounter()
	scrapeFailures   = metrics.NewCounter()
	csvParseFailures = metrics.NewCounter()
)

// Exported HAProxy metrics.
var (
	curQueue    = metrics.NewGauge()
	maxQueue    = metrics.NewGauge()
	curSessions = metrics.NewGauge()
	maxSessions = metrics.NewGauge()
	bytesIn     = metrics.NewGauge()
	bytesOut    = metrics.NewGauge()
	instanceUp  = metrics.NewGauge()
)

// Mappings from CSV field indexes to metrics.
var fieldToMetric = map[int]metrics.Gauge{
	2:  newGauge("haproxy_current_queue", "Current instance queue length."),
	3:  newGauge("haproxy_max_queue", "Maximum instance queue length."),
	4:  newGauge("haproxy_current_sessions", "Current number of sessions."),
	5:  newGauge("haproxy_max_sessions", "Maximum number of sessions."),
	8:  newGauge("haproxy_bytes_in", "Current total of incoming bytes."),
	9:  newGauge("haproxy_bytes_out", "Current total of outgoing bytes."),
	17: newGauge("haproxy_instance_up", "Current health status of the instance (1 = UP, 0 = DOWN)."),
}

func newGauge(metricName string, docString string) metrics.Gauge {
	gauge := metrics.NewGauge()
	registry.DefaultRegistry.Register(metricName, docString, registry.NilLabels, gauge)
	return gauge
}

func scrapePeriodically(csvRows chan []string) {
	for {
		scrapeOnce(csvRows)
		time.Sleep(*haProxyScrapeInterval * time.Second)
	}
}

func scrapeOnce(csvRows chan []string) {
	defer totalScrapes.Increment(registry.NilLabels)

	log.Printf("Scraping %s", *haProxyScrapeUri)
	resp, err := http.Get(*haProxyScrapeUri)
	if err != nil {
		log.Printf("Error while scraping HAProxy: %v", err)
		scrapeFailures.Increment(registry.NilLabels)
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
			csvParseFailures.Increment(registry.NilLabels)
			return
		}
		if len(row) == 0 {
			continue
		}

		csvRows <- row
	}
}

func exportMetrics(csvRows chan []string) {
	for {
		row := <-csvRows
		exportCsvRow(row)
	}
}

func exportCsvRow(csvRow []string) {
	if len(csvRow) != expectedCsvFieldCount {
		log.Printf("Wrong CSV field count: %i vs. %i", len(csvRow), expectedCsvFieldCount)
		csvParseFailures.Increment(registry.NilLabels)
		return
	}

	service, instance := csvRow[0], csvRow[1]
	if instance == "FRONTEND" || instance == "BACKEND" {
		// Ignore these summary rows for now, since we can aggregate by ourselves later.
		return
	}

	labels := map[string]string{
		"service":  service,
		"instance": instance,
	}

	for fieldIdx, gauge := range fieldToMetric {
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
		default:
			value, err = strconv.ParseInt(valueStr, 10, 64)
			if err != nil {
				log.Printf("Error while parsing CSV field value %s: %v", valueStr, err)
				csvParseFailures.Increment(registry.NilLabels)
				continue
			}
		}
		gauge.Set(labels, float64(value))
	}
}

func serveStatus() {
	exporter := registry.DefaultRegistry.YieldExporter()

	http.Handle(*metricsEndpoint, exporter)
	http.ListenAndServe(*listeningAddress, nil)
}

func main() {
	flag.Parse()

	registry.Register("haproxy_exporter_total_scrapes", "Current total HAProxy scrapes.", registry.NilLabels, scrapeFailures)
	registry.Register("haproxy_exporter_scrape_failures", "Number of errors while scraping HAProxy.", registry.NilLabels, scrapeFailures)
	registry.Register("haproxy_exporter_csv_parse_failures", "Number of errors while scraping HAProxy.", registry.NilLabels, csvParseFailures)

	csvRows := make(chan []string)

	go exportMetrics(csvRows)
	go serveStatus()
	scrapePeriodically(csvRows)
}
