package main

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
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

	e := NewExporter(h.URL, serverMetrics, 5*time.Second)
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
	h := newHaproxy([]byte("test,127.0.0.1:8080,0,0,0,0,,0,0,0,,0,,0,0,0,0,no check,1,1,0,0,,,0,,1,1,1,,0,,2,0,,0,,,,0,0,0,0,0,0,0,,,,0,0,"))
	defer h.Close()

	e := NewExporter(h.URL, serverMetrics, 5*time.Second)
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

	got := 0
	for range ch {
		got += 1
	}
	if expect := len(e.serverMetrics) - 1; got != expect {
		t.Errorf("expected %d metrics, got %d", expect, got)
	}
}

// TestServerBrokenCSV ensures bugs in CSV format are handled gracefully. List of known bugs:
//
//   * http://permalink.gmane.org/gmane.comp.web.haproxy/26561
//
func TestServerBrokenCSV(t *testing.T) {
	const data = `foo,FRONTEND,0,0,0,0,,0,0,0,,0,,0,0,0,0,UP,1,1,0,0,0,5007,0,,1,8,1,,0,,2,0,,0,L4OK,,0,,,,,,,0,,,,0,0,
foo,bug-missing-comma,0,0,0,0,,0,0,0,,0,,0,0,0,0,DRAIN (agent)1,1,0,0,0,5007,0,,1,8,1,,0,,2,0,,0,L4OK,,0,,,,,,,0,,,,0,0,
foo,foo-instance-0,0,0,0,0,,0,0,0,,0,,0,0,0,0,UP,1,1,0,0,0,5007,0,,1,8,1,,0,,2,0,,0,L4OK,,0,,,,,,,0,,,,0,0,
foo,BACKEND,0,0,0,0,,0,0,0,,0,,0,0,0,0,UP,1,1,0,0,0,5007,0,,1,8,1,,0,,2,0,,0,L4OK,,0,,,,,,,0,,,,0,0,
`
	h := newHaproxy([]byte(data))
	defer h.Close()

	e := NewExporter(h.URL, serverMetrics, 5*time.Second)
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

	got := 0
	for range ch {
		got += 1
	}
	if expect := len(e.frontendMetrics) + len(e.backendMetrics); got < expect {
		t.Errorf("expected at least %d metrics, got %d", expect, got)
	}
}

func TestConfigChangeDetection(t *testing.T) {
	h := newHaproxy([]byte(""))
	defer h.Close()

	e := NewExporter(h.URL, serverMetrics, 5*time.Second)
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

	e := NewExporter(s.URL, serverMetrics, 1*time.Second)
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

func TestNotFound(t *testing.T) {
	s := httptest.NewServer(http.NotFoundHandler())
	defer s.Close()

	e := NewExporter(s.URL, serverMetrics, 1*time.Second)
	ch := make(chan prometheus.Metric)
	go func() {
		defer close(ch)
		e.Collect(ch)
	}()

	if expect, got := 0., readGauge((<-ch).(prometheus.Gauge)); expect != got {
		// up
		t.Errorf("expected %f up, got %f", expect, got)
	}
}

func TestParseStatusField(t *testing.T) {
	tests := []struct {
		input string
		want  int64
	}{
		{"UP", 1},
		{"UP 1/3", 1},
		{"UP 2/3", 1},
		{"OPEN", 1},
		{"no check", 1},
		{"DOWN", 0},
		{"DOWN 1/2", 0},
		{"NOLB", 0},
		{"MAINT", 0}, // prometheus/haproxy_exporter#35
		{"unknown", 0},
	}

	for _, tt := range tests {
		if have := parseStatusField(tt.input); tt.want != have {
			t.Errorf("want status value %d for input %s, have %d",
				tt.want,
				tt.input,
				have,
			)
		}
	}
}

func TestFilterServerMetrics(t *testing.T) {
	tests := []struct {
		input string
		want  metricsMap
	}{
		{input: "", want: metricsMap{}},
		{input: "8", want: metricsMap{8: serverMetrics[8]}},
		{input: serverMetrics.String(), want: serverMetrics},
	}

	for _, tt := range tests {
		have, err := filterServerMetrics(tt.input)
		if err != nil {
			t.Errorf("unexpected error for input %s: %s", tt.input, err)
			continue
		}
		if !reflect.DeepEqual(tt.want, have) {
			t.Errorf("want filtered metrics %s for input %s, have %s",
				tt.want,
				tt.input,
				have,
			)
		}
	}
}

func BenchmarkExtract(b *testing.B) {
	config, err := ioutil.ReadFile("test/haproxy.csv")
	if err != nil {
		b.Fatalf("could not read config file: %v", err.Error())
	}

	h := newHaproxy(config)
	defer h.Close()

	e := NewExporter(h.URL, serverMetrics, 5*time.Second)

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
