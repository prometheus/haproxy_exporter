package main

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"regexp"
	"runtime"
	"strconv"
	"testing"

	dto "github.com/prometheus/client_model/go"

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
	re := regexp.MustCompile(`counter:<value:(\d+) >`)

	pb := &dto.Metric{}
	m.Write(pb)

	matches := re.FindStringSubmatch(pb.String())
	if len(matches) != 2 {
		return 0
	}

	v, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0
	}

	return v
}

func TestInvalidConfig(t *testing.T) {
	h := newHaproxy([]byte("not,enough,fields"))
	defer h.Close()

	e := NewExporter(h.URL)
	ch := make(chan prometheus.Metric)

	go func() {
		defer close(ch)
		e.Collect(ch)
	}()

	if expect, got := 1, readCounter((<-ch).(prometheus.Counter)); expect != got {
		// totalScrapes
		t.Errorf("expected %d recorded scrape, got %d", expect, got)
	}
	if expect, got := 0, readCounter((<-ch).(prometheus.Counter)); expect != got {
		// scrapeFailures
		t.Errorf("expected %d failed scrape, got %d", expect, got)
	}
	if expect, got := 1, readCounter((<-ch).(prometheus.Counter)); expect != got {
		// csvParseFailures
		t.Errorf("expected %d csv parse failures, got %d", expect, got)
	}
	if <-ch != nil {
		t.Errorf("expected closed channel")
	}
}

func TestServerWithoutChecks(t *testing.T) {
	h := newHaproxy([]byte("test,127.0.0.1:8080,0,0,0,0,,0,0,0,,0,,0,0,0,0,no check,1,1,0,,,,,,1,1,1,,0,,2,0,,0,,,,0,0,0,0,0,0,0,,,,0,0,"))
	defer h.Close()

	e := NewExporter(h.URL)
	ch := make(chan prometheus.Metric)

	go func() {
		defer close(ch)
		e.Collect(ch)
	}()

	if expect, got := 1, readCounter((<-ch).(prometheus.Counter)); expect != got {
		// totalScrapes
		t.Errorf("expected %d recorded scrape, got %d", expect, got)
	}
	if expect, got := 0, readCounter((<-ch).(prometheus.Counter)); expect != got {
		// scrapeFailures
		t.Errorf("expected %d failed scrape, got %d", expect, got)
	}
	if expect, got := 0, readCounter((<-ch).(prometheus.Counter)); expect != got {
		// csvParseFailures
		t.Errorf("expected %d csv parse failures, got %d", expect, got)
	}
	// Such up the remaining metrics.
	for _ = range ch {
	}
}

func BenchmarkExtract(b *testing.B) {
	config, err := ioutil.ReadFile("test/haproxy.csv")
	if err != nil {
		b.Fatalf("could not read config file: %v", err.Error())
	}

	h := newHaproxy(config)
	defer h.Close()

	e := NewExporter(h.URL)

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ch := make(chan prometheus.Metric)
		go func(ch chan prometheus.Metric) {
			for _ = range ch {
			}
		}(ch)

		e.Collect(ch)
		close(ch)
	}

	runtime.GC()
	runtime.ReadMemStats(&after)

	b.Logf("%d bytes used after %d runs", after.Alloc-before.Alloc, b.N)
}
