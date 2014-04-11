package main

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"regexp"
	"runtime"
	"strconv"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

type haproxy struct {
	*httptest.Server
	config []byte
}

func newHaproxy(config []byte) *haproxy {
	h := &haproxy{config: config}
	h.Server = httptest.NewServer(handler(h))
	return h
}

func handler(h *haproxy) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Write(h.config)
	}
}

func readCounter(m prometheus.Counter) int {
	// seriously? prometheus isn't very good in exposing metrics ...
	re := regexp.MustCompile(`&{map\[\] (\d+)\}`)

	matches := re.FindStringSubmatch(m.String())
	if len(matches) != 2 {
		return 0
	}

	v, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0
	}

	return v
}

func resetGlobalCounters() {
	totalScrapes.ResetAll()
	scrapeFailures.ResetAll()
	csvParseFailures.ResetAll()
}

func TestInvalidConfig(t *testing.T) {
	h := newHaproxy([]byte("not,enough,fields"))
	defer h.Close()

	resetGlobalCounters()
	e := NewExporter(h.URL)
	e.Scrape()

	if expect, got := 1, readCounter(totalScrapes); expect != got {
		t.Errorf("expected %d recorded scrape, got %d", expect, got)
	}

	if expect, got := 1, readCounter(csvParseFailures); expect != got {
		t.Errorf("expected %d failed scrape, got %d", expect, got)
	}
}

func TestServerWithoutChecks(t *testing.T) {
	h := newHaproxy([]byte("test,127.0.0.1:8080,0,0,0,0,,0,0,0,,0,,0,0,0,0,no check,1,1,0,,,,,,1,1,1,,0,,2,0,,0,,,,0,0,0,0,0,0,0,,,,0,0,"))
	defer h.Close()

	resetGlobalCounters()
	e := NewExporter(h.URL)
	e.Scrape()

	if expect, got := 1, readCounter(totalScrapes); expect != got {
		t.Errorf("expected %d recorded scrape, got %d", expect, got)
	}

	if expect, got := 0, readCounter(csvParseFailures); expect != got {
		t.Errorf("expected %d failed parsings, got %d", expect, got)
	}
}

func TestConfigChangeDetection(t *testing.T) {
	h := newHaproxy([]byte(""))
	defer h.Close()

	resetGlobalCounters()
	e := NewExporter(h.URL)
	e.Scrape()

	// TODO: Add a proper test here. Unfortunately, it's not possible to get any
	// numbers out of the registry, once added. The overhead of parsing the
	// JSON/protobuf output and testing the correct results here is currently
	// deemed too high, given the imminent client rewrite.
}

func BenchmarkExtract(b *testing.B) {
	config, err := ioutil.ReadFile("test/haproxy.csv")
	if err != nil {
		b.Fatalf("could not read config file: %v", err.Error())
	}

	h := newHaproxy(config)
	defer h.Close()

	resetGlobalCounters()
	e := NewExporter(h.URL)

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		e.Scrape()
	}

	runtime.GC()
	runtime.ReadMemStats(&after)

	b.Logf("%d bytes used after %d runs", after.Alloc-before.Alloc, b.N)
}
