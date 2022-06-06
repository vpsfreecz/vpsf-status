package main

import (
	"log"
	"net/http"
	"time"
)

func checkVpsAdminWebServices(st *Status, checkInterval time.Duration) {
	go spawnHttpCheck(st.VpsAdmin.Webui, checkInterval)
	go spawnHttpCheck(st.VpsAdmin.Console, checkInterval)
}

func checkWebServices(st *Status, checkInterval time.Duration) {
	for _, ws := range st.Services.Web {
		go spawnHttpCheck(ws, checkInterval)
	}
}

func spawnHttpCheck(ws *WebService, checkInterval time.Duration) {
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
			ws.StatusString = resp.Status

			if resp.StatusCode != 200 {
				log.Printf("Failed to check %s: got HTTP %s", ws.Label, resp.Status)
				ws.Status = false
				return
			}

			ws.Status = true
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
