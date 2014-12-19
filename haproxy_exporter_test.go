package main

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"

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

func handlerStale(exit chan bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		<-exit
	}
}

func readCounter(m prometheus.Counter) float64 {
	// TODO: Revisit this once client_golang offers better testing tools.
	pb := &dto.Metric{}
	m.Write(pb)
	return pb.GetCounter().GetValue()
}

func readGauge(m prometheus.Gauge) float64 {
	// TODO: Revisit this once client_golang offers better testing tools.
	pb := &dto.Metric{}
	m.Write(pb)
	return pb.GetGauge().GetValue()
}

func TestInvalidConfig(t *testing.T) {
	h := newHaproxy([]byte("not,enough,fields"))
	defer h.Close()

	e := NewExporter(h.URL, "", 5*time.Second)
	ch := make(chan prometheus.Metric)

	go func() {
		defer close(ch)
		e.Collect(ch)
	}()

	if expect, got := 1., readGauge((<-ch).(prometheus.Gauge)); expect != got {
		// up
		t.Errorf("expected %f up, got %f", expect, got)
	}
	if expect, got := 1., readCounter((<-ch).(prometheus.Counter)); expect != got {
		// totalScrapes
		t.Errorf("expected %f recorded scrape, got %f", expect, got)
	}
	if expect, got := 1., readCounter((<-ch).(prometheus.Counter)); expect != got {
		// csvParseFailures
		t.Errorf("expected %f csv parse failures, got %f", expect, got)
	}
	if <-ch != nil {
		t.Errorf("expected closed channel")
	}
}

func TestServerWithoutChecks(t *testing.T) {
	h := newHaproxy([]byte("test,127.0.0.1:8080,0,0,0,0,,0,0,0,,0,,0,0,0,0,no check,1,1,0,,,,,,1,1,1,,0,,2,0,,0,,,,0,0,0,0,0,0,0,,,,0,0,"))
	defer h.Close()

	e := NewExporter(h.URL, "", 5*time.Second)
	ch := make(chan prometheus.Metric)

	go func() {
		defer close(ch)
		e.Collect(ch)
	}()

	if expect, got := 1., readGauge((<-ch).(prometheus.Gauge)); expect != got {
		// up
		t.Errorf("expected %f up, got %f", expect, got)
	}
	if expect, got := 1., readCounter((<-ch).(prometheus.Counter)); expect != got {
		// totalScrapes
		t.Errorf("expected %f recorded scrape, got %f", expect, got)
	}
	if expect, got := 0., readCounter((<-ch).(prometheus.Counter)); expect != got {
		// csvParseFailures
		t.Errorf("expected %f csv parse failures, got %f", expect, got)
	}
	// Suck up the remaining metrics.
	for _ = range ch {
	}
}

func TestConfigChangeDetection(t *testing.T) {
	h := newHaproxy([]byte(""))
	defer h.Close()

	e := NewExporter(h.URL, "", 5*time.Second)
	ch := make(chan prometheus.Metric)

	go func() {
		defer close(ch)
		e.Collect(ch)
	}()

	// TODO: Add a proper test here. Vet the possibilities of the new
	// client_golang to do this easily. If better test support is needed,
	// add it to client_golang first. (See also readCounter() above.)

	// Suck up the remaining metrics.
	for _ = range ch {
	}
}

func TestDeadline(t *testing.T) {
	exit := make(chan bool)
	s := httptest.NewServer(handlerStale(exit))
	defer func() {
		// s.Close() will block until the handler
		// returns, so we need to make it exit.
		exit <- true
		s.Close()
	}()

	e := NewExporter(s.URL, "", 1*time.Second)
	ch := make(chan prometheus.Metric)
	go func() {
		defer close(ch)
		e.Collect(ch)
	}()

	if expect, got := 0., readGauge((<-ch).(prometheus.Gauge)); expect != got {
		// up
		t.Errorf("expected %f up, got %f", expect, got)
	}
	if expect, got := 1., readCounter((<-ch).(prometheus.Counter)); expect != got {
		// totalScrapes
		t.Errorf("expected %f recorded scrape, got %f", expect, got)
	}
	if expect, got := 0., readCounter((<-ch).(prometheus.Counter)); expect != got {
		// csvParseFailures
		t.Errorf("expected %f csv parse failures, got %f", expect, got)
	}
	if <-ch != nil {
		t.Errorf("expected closed channel")
	}
}

func BenchmarkExtract(b *testing.B) {
	config, err := ioutil.ReadFile("test/haproxy.csv")
	if err != nil {
		b.Fatalf("could not read config file: %v", err.Error())
	}

	h := newHaproxy(config)
	defer h.Close()

	e := NewExporter(h.URL, "", 5*time.Second)

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
