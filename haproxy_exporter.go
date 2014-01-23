package main

import (
	"encoding/csv"
	"flag"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Constants.
const expectedCsvFieldCount = 52

// Commandline flags.
var (
	listeningAddress      = flag.String("telemetry.address", ":8080", "Address on which to expose JSON metrics.")
	metricsEndpoint       = flag.String("telemetry.endpoint", prometheus.ExpositionResource, "Path under which to expose metrics.")
	haProxyScrapeUri      = flag.String("haproxy.scrape_uri", "http://localhost/;csv", "URI on which to scrape HAProxy.")
	haProxyScrapeInterval = flag.Duration("haproxy.scrape_interval", 15, "Interval in seconds between scrapes.")
)

// Exported internal metrics.
var (
	totalScrapes     = prometheus.NewCounter()
	scrapeFailures   = prometheus.NewCounter()
	csvParseFailures = prometheus.NewCounter()
)

// Mappings from CSV summary field indexes to metrics.
var summaryFieldToMetric = map[int]prometheus.Gauge{
	2: newGauge("haproxy_current_queue", "Current server queue length."),
	3: newGauge("haproxy_max_queue", "Maximum server queue length."),
}

// Mappings from CSV field indexes to metrics.
var fieldToMetric = map[int]prometheus.Gauge{
	4:  newGauge("haproxy_current_sessions", "Current number of active sessions."),
	5:  newGauge("haproxy_max_sessions", "Maximum number of active sessions."),
	8:  newGauge("haproxy_bytes_in", "Current total of incoming bytes."),
	9:  newGauge("haproxy_bytes_out", "Current total of outgoing bytes."),
	17: newGauge("haproxy_instance_up", "Current health status of the instance (1 = UP, 0 = DOWN)."),
	33: newGauge("haproxy_current_session_rate", "Current number of sessions per second."),
	35: newGauge("haproxy_max_session_rate", "Maximum number of sessions per second."),
}

func newGauge(metricName string, docString string) prometheus.Gauge {
	gauge := prometheus.NewGauge()
	prometheus.Register(metricName, docString, prometheus.NilLabels, gauge)
	return gauge
}

func scrapePeriodically(csvRows chan []string) {
	for {
		scrapeOnce(csvRows)
		time.Sleep(*haProxyScrapeInterval * time.Second)
	}
}

func scrapeOnce(csvRows chan []string) {
	defer totalScrapes.Increment(prometheus.NilLabels)

	log.Printf("Scraping %s", *haProxyScrapeUri)
	resp, err := http.Get(*haProxyScrapeUri)
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

func exportMetrics(csvRows chan []string) {
	for {
		row := <-csvRows
		exportCsvRow(row)
	}
}

func exportCsvRow(csvRow []string) {
	if len(csvRow) != expectedCsvFieldCount {
		log.Printf("Wrong CSV field count: %i vs. %i", len(csvRow), expectedCsvFieldCount)
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

		exportCsvFields(labels, summaryFieldToMetric, csvRow)
	} else {
		labels := map[string]string{
			"service":  service,
			"instance": instance,
		}

		exportCsvFields(labels, fieldToMetric, csvRow)
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
	flag.Parse()

	prometheus.Register("haproxy_exporter_total_scrapes", "Current total HAProxy scrapes.", prometheus.NilLabels, scrapeFailures)
	prometheus.Register("haproxy_exporter_scrape_failures", "Number of errors while scraping HAProxy.", prometheus.NilLabels, scrapeFailures)
	prometheus.Register("haproxy_exporter_csv_parse_failures", "Number of errors while scraping HAProxy.", prometheus.NilLabels, csvParseFailures)

	csvRows := make(chan []string)
	go scrapePeriodically(csvRows)
	go exportMetrics(csvRows)

	log.Printf("Starting Server: %s", *listeningAddress)
	http.Handle(*metricsEndpoint, prometheus.DefaultHandler)
	log.Fatal(http.ListenAndServe(*listeningAddress, nil))
}
