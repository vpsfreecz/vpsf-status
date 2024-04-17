package main

import (
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func checkVpsAdminWebServices(st *Status, checkInterval time.Duration) {
	go spawnHttpCheck(
		st.VpsAdmin.Webui,
		st.Exporter.vpsAdminStatus.With(prometheus.Labels{"service": "webui"}),
		checkInterval,
	)

	go spawnHttpCheck(
		st.VpsAdmin.Console,
		st.Exporter.vpsAdminStatus.With(prometheus.Labels{"service": "console"}),
		checkInterval,
	)
}

func checkWebServices(st *Status, checkInterval time.Duration) {
	for _, ws := range st.Services.Web {
		go spawnHttpCheck(
			ws,
			st.Exporter.webServiceStatus.With(prometheus.Labels{"service": ws.Label}),
			checkInterval,
		)
	}
}

func spawnHttpCheck(ws *WebService, gauge prometheus.Gauge, checkInterval time.Duration) {
	for {
		ws.LastCheck = time.Now()

		resp, err := sendHttpRequest(ws)
		if err != nil {
			log.Printf("Unable to check %s: %+v", ws.Label, err)
			ws.Status = false
			time.Sleep(checkInterval)
			continue
		}

		func() {
			defer resp.Body.Close()

			ws.StatusCode = resp.StatusCode

			if resp.StatusCode == 200 {
				ws.Status = true
				ws.Maintenance = false
				gauge.Set(0)
				return
			} else if resp.StatusCode == 503 {
				ws.Status = false
				ws.Maintenance = true
				gauge.Set(1)
				return
			}

			log.Printf("Failed to check %s: got HTTP %s", ws.Label, resp.Status)
			ws.Status = false
			ws.Maintenance = false
			gauge.Set(2)
		}()

		time.Sleep(checkInterval)
	}
}

func sendHttpRequest(ws *WebService) (*http.Response, error) {
	if ws.Method == "get" {
		return http.Get(ws.Url)
	}

	return http.Head(ws.Url)
}
