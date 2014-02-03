package main

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"runtime"
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
	l := prometheus.NilLabels
	m.Increment(l)
	return int(m.Decrement(l))
}

func TestInvalidConfig(t *testing.T) {
	h := newHaproxy([]byte("not,enough,fields"))
	defer h.Close()

	e := NewExporter(h.URL)
	e.Scrape()

	if expect, got := 1, readCounter(totalScrapes); expect != got {
		t.Errorf("expected %d recorded scrape, got %d", expect, got)
	}

	if expect, got := 1, readCounter(scrapeFailures); expect != got {
		t.Errorf("expected %d failed scrape, got %d", expect, got)
	}
}

func TestConfigChangeDetection(t *testing.T) {
	h := newHaproxy([]byte(""))
	defer h.Close()

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
