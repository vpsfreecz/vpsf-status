package main

import (
	"log"
	"time"

	"github.com/vpsfreecz/vpsadmin-go-client/client"
)

func checkApi(st *Status) {
	api := client.New(st.VpsAdmin.Api.Url)

	for {
		publicStatus := api.Node.PublicStatus.Prepare()
		now := time.Now()
		st.VpsAdmin.Api.LastCheck = now

		resp, err := publicStatus.Call()

		if err != nil {
			log.Printf("Unable to check API: %+v", err)
			failApi(st, now)
			time.Sleep(30 * time.Second)
			continue
		} else if !resp.Status {
			log.Printf("Failed to list nodes: %s", resp.Message)
			failApi(st, now)
			time.Sleep(30 * time.Second)
			continue
		}

		st.VpsAdmin.Api.Status = true

		for _, node := range resp.Output {
			updateNode(node, st, now)
		}

		time.Sleep(30 * time.Second)
	}
}

func failApi(st *Status, now time.Time) {
	st.VpsAdmin.Api.Status = false

	for _, loc := range st.LocationList {
		for _, node := range loc.NodeList {
			node.ApiStatus = false
			node.LastApiCheck = now
		}
	}
}

func updateNode(apiNode *client.ActionNodePublicStatusOutput, st *Status, now time.Time) {
	stLoc := st.LocationMap[apiNode.Location.Label]
	if stLoc == nil {
		log.Printf("Not configured for location %s", apiNode.Location.Label)
		return
	}

	stNode := stLoc.NodeMap[apiNode.Name]
	if stNode == nil {
		log.Printf("Not configured for node %s", apiNode.Name)
		return
	}

	stNode.LastApiCheck = now

	if apiNode.MaintenanceLock != "no" {
		stNode.ApiStatus = true
		stNode.ApiMaintenance = true
		return
	}

	stNode.ApiMaintenance = false

	lastReport, err := time.Parse("2006-01-02T15:04:05Z", apiNode.LastReport)
	if err != nil {
		log.Printf("Unable to parse node last_report of %v", apiNode.LastReport)
		stNode.ApiStatus = false
		return
	}

	diff := now.Sub(lastReport)
	stNode.ApiStatus = diff <= (150 * time.Second)
}
