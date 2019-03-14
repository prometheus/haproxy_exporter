// Copyright 2018 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bufio"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

const testSocket = "/tmp/haproxyexportertest.sock"

type haproxy struct {
	*httptest.Server
	response []byte
}

func newHaproxy(response []byte) *haproxy {
	h := &haproxy{response: response}
	h.Server = httptest.NewServer(handler(h))
	return h
}

func handler(h *haproxy) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Write(h.response)
	}
}

func handlerStale(exit chan bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		<-exit
	}
}

func expectMetrics(t *testing.T, c prometheus.Collector, fixture string) {
	exp, err := os.Open(path.Join("test", fixture))
	if err != nil {
		t.Fatalf("Error opening fixture file %q: %v", fixture, err)
	}
	if err := testutil.CollectAndCompare(c, exp); err != nil {
		t.Fatal("Unexpected metrics returned:", err)
	}
}

func TestInvalidConfig(t *testing.T) {
	h := newHaproxy([]byte("not,enough,fields"))
	defer h.Close()

	e, _ := NewExporter(h.URL, true, 5*time.Second, nil)

	expectMetrics(t, e, "invalid_config.metrics")
}

func TestServerWithoutChecks(t *testing.T) {
	h := newHaproxy([]byte("test,127.0.0.1:8080,0,0,0,0,0,0,0,0,,0,,0,0,0,0,no check,1,1,0,0,,,0,,1,1,1,,0,,2,0,,0,,,,0,0,0,0,0,0,0,,,,0,0,,,,,,,,,,,"))
	defer h.Close()

	e, _ := NewExporter(h.URL, true, 5*time.Second, nil)

	expectMetrics(t, e, "server_without_checks.metrics")
}

// TestServerBrokenCSV ensures bugs in CSV format are handled gracefully. List of known bugs:
//
//   * http://permalink.gmane.org/gmane.comp.web.haproxy/26561
//
func TestServerBrokenCSV(t *testing.T) {
	const data = `foo,FRONTEND,0,0,0,0,,0,0,0,,0,,0,0,0,0,UP,1,1,0,0,0,5007,0,,1,8,1,,0,,2,0,,0,L4OK,,0,,,,,,,0,,,,0,0,,,,,,,,,,,
foo,bug-missing-comma,0,0,0,0,,0,0,0,,0,,0,0,0,0,DRAIN (agent)1,1,0,0,0,5007,0,,1,8,1,,0,,2,0,,0,L4OK,,0,,,,,,,0,,,,0,0,,,,,,,,,,,
foo,foo-instance-0,0,0,0,0,,0,0,0,,0,,0,0,0,0,UP,1,1,0,0,0,5007,0,,1,8,1,,0,,2,0,,0,L4OK,,0,,,,,,,0,,,,0,0,,,,,,,,,,,
foo,BACKEND,0,0,0,0,,0,0,0,,0,,0,0,0,0,UP,1,1,0,0,0,5007,0,,1,8,1,,0,,2,0,,0,L4OK,,0,,,,,,,0,,,,0,0,,,,,,,,,,,
`
	h := newHaproxy([]byte(data))
	defer h.Close()

	e, _ := NewExporter(h.URL, true, 5*time.Second, nil)

	expectMetrics(t, e, "server_broken_csv.metrics")
}

func TestOlderHaproxyVersions(t *testing.T) {
	const data = `foo,FRONTEND,0,0,0,0,,0,0,0,,0,,0,0,0,0,UP,1,1,0,0,0,5007,0,,1,8,1,,0,,2,
foo,foo-instance-0,0,0,0,0,,0,0,0,,0,,0,0,0,0,UP,1,1,0,0,0,5007,0,,1,8,1,,0,,2,
foo,BACKEND,0,0,0,0,,0,0,0,,0,,0,0,0,0,UP,1,1,0,0,0,5007,0,,1,8,1,,0,,2,
`
	h := newHaproxy([]byte(data))
	defer h.Close()

	e, _ := NewExporter(h.URL, true, 5*time.Second, nil)

	expectMetrics(t, e, "older_haproxy_versions.metrics")
}

func TestConfigChangeDetection(t *testing.T) {
	h := newHaproxy([]byte(""))
	defer h.Close()

	e, _ := NewExporter(h.URL, true, 5*time.Second, nil)
	ch := make(chan prometheus.Metric)

	go func() {
		defer close(ch)
		e.Collect(ch)
	}()

	// TODO: Add a proper test here. Vet the possibilities of the new
	// client_golang to do this easily. If better test support is needed,
	// add it to client_golang first. (See also readCounter() above.)

	// Suck up the remaining metrics.
	for range ch {
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

	e, err := NewExporter(s.URL, true, 1*time.Second, nil)
	if err != nil {
		t.Fatal(err)
	}

	expectMetrics(t, e, "deadline.metrics")
}

func TestNotFound(t *testing.T) {
	s := httptest.NewServer(http.NotFoundHandler())
	defer s.Close()

	e, err := NewExporter(s.URL, true, 1*time.Second, nil)
	if err != nil {
		t.Fatal(err)
	}

	expectMetrics(t, e, "not_found.metrics")
}

func newHaproxyUnix(file, statsPayload string) (io.Closer, error) {
	if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	l, err := net.Listen("unix", file)
	if err != nil {
		return nil, err
	}
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				r := bufio.NewReader(c)
				for {
					l, err := r.ReadString('\n')
					if err != nil {
						return
					}
					switch l {
					case "show stat\n":
						c.Write([]byte(statsPayload))
						return
					default:
						// invalid command
						return
					}
				}
			}(c)
		}
	}()
	return l, nil
}

func TestUnixDomain(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("not on windows")
		return
	}
	srv, err := newHaproxyUnix(testSocket, "test,127.0.0.1:8080,0,0,0,0,0,0,0,0,,0,,0,0,0,0,no check,1,1,0,0,,,0,,1,1,1,,0,,2,0,,0,,,,0,0,0,0,0,0,0,,,,0,0,,,,,,,,,,,\n")
	if err != nil {
		t.Fatalf("can't start test server: %v", err)
	}
	defer srv.Close()

	e, err := NewExporter("unix:"+testSocket, true, 5*time.Second, nil)
	if err != nil {
		t.Fatal(err)
	}

	expectMetrics(t, e, "unix_domain.metrics")
}

func TestUnixDomainNotFound(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("not on windows")
		return
	}

	if err := os.Remove(testSocket); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	e, _ := NewExporter("unix:"+testSocket, true, 1*time.Second, nil)
	expectMetrics(t, e, "unix_domain_not_found.metrics")
}

func TestUnixDomainDeadline(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("not on windows")
		return
	}

	exit := make(chan struct{})
	defer close(exit)

	if err := os.Remove(testSocket); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	l, err := net.Listen("unix", testSocket)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	go func() {
		for {
			if _, err := l.Accept(); err != nil {
				return
			}
			go func() {
				// block
				<-exit
			}()
		}
	}()

	e, _ := NewExporter("unix:"+testSocket, true, 1*time.Second, nil)

	expectMetrics(t, e, "unix_domain_deadline.metrics")
}

func TestInvalidScheme(t *testing.T) {
	e, err := NewExporter("gopher://gopher.quux.org", true, 1*time.Second, nil)
	if expect, got := (*Exporter)(nil), e; expect != got {
		t.Errorf("expected %v, got %v", expect, got)
	}
	if err == nil {
		t.Fatalf("expected non-nil error")
	}
	if expect, got := err.Error(), `unsupported scheme: "gopher"`; expect != got {
		t.Errorf("expected %q, got %q", expect, got)
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
	config, err := ioutil.ReadFile("test/haproxy.csv")
	if err != nil {
		t.Fatalf("could not read config file: %v", err.Error())
	}

	h := newHaproxy(config)
	defer h.Close()

	exporter, _ := NewExporter(h.URL, true, 5*time.Second, nil)
	tests := []struct {
		input string
		want  map[int]*prometheus.Desc
	}{
		{input: "", want: map[int]*prometheus.Desc{}},
		{input: "8", want: map[int]*prometheus.Desc{8: exporter.serverMetrics[8]}},
		{input: serverMetricsString, want: exporter.serverMetrics},
	}
	for _, tt := range tests {
		e, _ := NewExporter(h.URL, true, 5*time.Second, nil)

		err := e.filterServerMetrics(tt.input)
		if err != nil {
			t.Errorf("unexpected error for input %s: %s", tt.input, err)
			continue
		}
		if !reflect.DeepEqual(tt.want, e.serverMetrics) {
			t.Errorf("want filtered metrics %+v for input %q, have %+v",
				tt.want,
				tt.input,
				e.serverMetrics,
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

	e, _ := NewExporter(h.URL, true, 5*time.Second, nil)

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ch := make(chan prometheus.Metric)
		go func(ch chan prometheus.Metric) {
			for range ch {
			}
		}(ch)

		e.Collect(ch)
		close(ch)
	}

	runtime.GC()
	runtime.ReadMemStats(&after)

	b.Logf("%d bytes used after %d runs", after.Alloc-before.Alloc, b.N)
}
