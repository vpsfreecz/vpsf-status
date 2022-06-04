package main

import (
	"log"
	"net/http"
	"time"
)

func checkVpsAdminWebServices(st *Status) {
	go spawnHttpCheck(st.VpsAdmin.Webui)
	go spawnHttpCheck(st.VpsAdmin.Console)
}

func checkWebServices(st *Status) {
	for _, ws := range st.Services.Web {
		go spawnHttpCheck(ws)
	}
}

func spawnHttpCheck(ws *WebService) {
	for {
		ws.LastCheck = time.Now()

		resp, err := sendHttpRequest(ws)
		if err != nil {
			log.Printf("Unable to check %s: %+v", ws.Label, err)
			ws.Status = false
			time.Sleep(30 * time.Second)
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

		time.Sleep(30 * time.Second)
	}
}

func sendHttpRequest(ws *WebService) (*http.Response, error) {
	if ws.Method == "get" {
		return http.Get(ws.Url)
	}

	return http.Head(ws.Url)
}
