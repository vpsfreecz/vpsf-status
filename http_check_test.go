package main

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type fakeHTTPDoer struct {
	statusCode int
	err        error
	requests   []*http.Request
}

func (f *fakeHTTPDoer) Do(req *http.Request) (*http.Response, error) {
	f.requests = append(f.requests, req)
	if f.err != nil {
		return nil, f.err
	}

	return &http.Response{
		StatusCode: f.statusCode,
		Status:     http.StatusText(f.statusCode),
		Body:       io.NopCloser(strings.NewReader("")),
	}, nil
}

func TestCheckHTTPOnce(t *testing.T) {
	tests := []struct {
		name        string
		statusCode  int
		err         error
		wantStatus  bool
		wantMaint   bool
		wantCode    int
		wantGauge   float64
		wantMethod  string
		method      string
		wantRequest string
	}{
		{
			name:        "200",
			statusCode:  http.StatusOK,
			wantStatus:  true,
			wantCode:    http.StatusOK,
			wantGauge:   0,
			wantMethod:  http.MethodHead,
			wantRequest: "https://check.example/status",
		},
		{
			name:        "503",
			statusCode:  http.StatusServiceUnavailable,
			wantMaint:   true,
			wantCode:    http.StatusServiceUnavailable,
			wantGauge:   1,
			wantMethod:  http.MethodHead,
			wantRequest: "https://check.example/status",
		},
		{
			name:        "non 200",
			statusCode:  http.StatusNotFound,
			wantCode:    http.StatusNotFound,
			wantGauge:   2,
			wantMethod:  http.MethodHead,
			wantRequest: "https://check.example/status",
		},
		{
			name:        "request error",
			err:         errors.New("dial failed"),
			wantCode:    0,
			wantGauge:   2,
			wantMethod:  http.MethodHead,
			wantRequest: "https://check.example/status",
		},
		{
			name:        "GET method",
			statusCode:  http.StatusOK,
			wantStatus:  true,
			wantCode:    http.StatusOK,
			wantGauge:   0,
			wantMethod:  http.MethodGet,
			method:      "get",
			wantRequest: "https://check.example/status",
		},
		{
			name:        "fallback to display URL",
			statusCode:  http.StatusOK,
			wantStatus:  true,
			wantCode:    http.StatusOK,
			wantGauge:   0,
			wantMethod:  http.MethodHead,
			wantRequest: "https://display.example/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ws := &WebService{
				Label:    "service",
				Url:      "https://display.example/",
				CheckUrl: "https://check.example/status",
				Method:   tt.method,
			}
			if tt.name == "fallback to display URL" {
				ws.CheckUrl = ""
			}

			client := &fakeHTTPDoer{statusCode: tt.statusCode, err: tt.err}
			gauge := prometheus.NewGauge(prometheus.GaugeOpts{Name: "test_http_status"})

			checkHTTPOnce(ws, gauge, client, fixedNow)

			if ws.Status != tt.wantStatus || ws.Maintenance != tt.wantMaint || ws.StatusCode != tt.wantCode {
				t.Fatalf("web service = %+v, want status=%v maintenance=%v code=%d", ws, tt.wantStatus, tt.wantMaint, tt.wantCode)
			}
			if !ws.LastCheck.Equal(fixedNow) {
				t.Fatalf("LastCheck = %s, want %s", ws.LastCheck, fixedNow)
			}
			if got := gaugeValue(t, gauge); got != tt.wantGauge {
				t.Fatalf("gauge = %v, want %v", got, tt.wantGauge)
			}
			if len(client.requests) != 1 {
				t.Fatalf("requests = %d, want 1", len(client.requests))
			}
			req := client.requests[0]
			if req.Method != tt.wantMethod {
				t.Fatalf("method = %s, want %s", req.Method, tt.wantMethod)
			}
			if req.URL.String() != tt.wantRequest {
				t.Fatalf("request URL = %s, want %s", req.URL.String(), tt.wantRequest)
			}
			if !req.Close {
				t.Fatal("request should ask the transport to close the connection")
			}
			if ws.Url != "https://display.example/" {
				t.Fatalf("display URL changed to %s", ws.Url)
			}
		})
	}
}

func TestNewHTTPCheckClientDoesNotReuseConnections(t *testing.T) {
	var newConnections int32

	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	server.Config.ConnState = func(conn net.Conn, state http.ConnState) {
		if state == http.StateNew {
			atomic.AddInt32(&newConnections, 1)
		}
	}
	server.Start()
	defer server.Close()

	ws := &WebService{
		Label:    "service",
		Url:      server.URL,
		CheckUrl: server.URL,
	}
	client := newHTTPCheckClient(5 * time.Second)
	gauge := prometheus.NewGauge(prometheus.GaugeOpts{Name: "test_no_reuse_http_status"})

	checkHTTPOnce(ws, gauge, client, fixedNow)
	checkHTTPOnce(ws, gauge, client, fixedNow.Add(time.Second))

	if got := atomic.LoadInt32(&newConnections); got != 2 {
		t.Fatalf("connections = %d, want 2", got)
	}
	if client.Timeout != 5*time.Second {
		t.Fatalf("client timeout = %s, want 5s", client.Timeout)
	}
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport type = %T, want *http.Transport", client.Transport)
	}
	if !transport.DisableKeepAlives {
		t.Fatal("transport should disable keep-alives")
	}
}

func TestCheckHTTPOnceHandlesInvalidURLWithoutRequest(t *testing.T) {
	ws := &WebService{
		Label:    "service",
		Url:      "https://display.example/",
		CheckUrl: "http://[::1",
	}
	client := &fakeHTTPDoer{statusCode: http.StatusOK}
	gauge := prometheus.NewGauge(prometheus.GaugeOpts{Name: "test_invalid_http_status"})

	checkHTTPOnce(ws, gauge, client, fixedNow)

	if ws.Status || ws.Maintenance || ws.StatusCode != 0 {
		t.Fatalf("web service = %+v, want down with no status code", ws)
	}
	if !ws.LastCheck.Equal(fixedNow) {
		t.Fatalf("LastCheck = %s, want %s", ws.LastCheck, fixedNow)
	}
	if got := gaugeValue(t, gauge); got != 2 {
		t.Fatalf("gauge = %v, want 2", got)
	}
	if len(client.requests) != 0 {
		t.Fatalf("requests = %d, want 0", len(client.requests))
	}
}
