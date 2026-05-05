package main

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type httpRequestDoer interface {
	Do(*http.Request) (*http.Response, error)
}

func checkVpsAdminWebServices(st *Status, checkInterval time.Duration, checkTimeout time.Duration) {
	go spawnHttpCheck(
		st,
		st.VpsAdmin.Webui,
		ProbeTarget{EntityKind: historyEntityVpsAdmin, EntityID: "webui", EntityLabel: vpsAdminServiceLabel("webui"), Method: "HTTP"},
		st.Exporter.vpsAdminStatus.With(prometheus.Labels{"service": "webui"}),
		checkInterval,
		checkTimeout,
	)

	go spawnHttpCheck(
		st,
		st.VpsAdmin.Console,
		ProbeTarget{EntityKind: historyEntityVpsAdmin, EntityID: "console", EntityLabel: vpsAdminServiceLabel("console"), Method: "HTTP"},
		st.Exporter.vpsAdminStatus.With(prometheus.Labels{"service": "console"}),
		checkInterval,
		checkTimeout,
	)
}

func checkWebServices(st *Status, checkInterval time.Duration, checkTimeout time.Duration) {
	for _, ws := range st.Services.Web {
		go spawnHttpCheck(
			st,
			ws,
			ProbeTarget{EntityKind: historyEntityWebService, EntityID: ws.Label, EntityLabel: ws.Label, Method: "HTTP"},
			st.Exporter.webServiceStatus.With(prometheus.Labels{"service": ws.Label}),
			checkInterval,
			checkTimeout,
		)
	}
}

func spawnHttpCheck(st *Status, ws *WebService, target ProbeTarget, gauge prometheus.Gauge, checkInterval time.Duration, checkTimeout time.Duration) {
	client := newHTTPCheckClient(checkTimeout)
	for {
		now := time.Now()
		checkHTTPOnce(ws, gauge, client, now)
		status, message := webServiceProbeStatus(ws)
		recordProbeStatus(st, target, status, message, now)
		sleepUntilNextProbe(now, checkInterval)
	}
}

func newHTTPCheckClient(checkTimeout time.Duration) *http.Client {
	return newProbeHTTPClient(checkTimeout)
}

func checkHTTPOnce(ws *WebService, gauge prometheus.Gauge, client httpRequestDoer, now time.Time) {
	ws.LastCheck = now

	resp, err := sendHttpRequest(client, ws)
	if err != nil {
		log.Printf("Unable to check %s: %+v", ws.Label, err)
		ws.Status = false
		ws.Maintenance = false
		ws.StatusCode = 0
		gauge.Set(2)
		return
	}

	defer resp.Body.Close()

	ws.StatusCode = resp.StatusCode

	if resp.StatusCode == http.StatusOK {
		ws.Status = true
		ws.Maintenance = false
		gauge.Set(0)
		return
	} else if resp.StatusCode == http.StatusServiceUnavailable {
		ws.Status = false
		ws.Maintenance = true
		gauge.Set(1)
		return
	}

	log.Printf("Failed to check %s: got HTTP %s", ws.Label, resp.Status)
	ws.Status = false
	ws.Maintenance = false
	gauge.Set(2)
}

func sendHttpRequest(client httpRequestDoer, ws *WebService) (*http.Response, error) {
	method := http.MethodHead
	if strings.ToLower(ws.Method) == "get" {
		method = http.MethodGet
	}

	url := ws.CheckUrl
	if url == "" {
		url = ws.Url
	}

	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}
	req.Close = true

	return client.Do(req)
}
