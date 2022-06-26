package main

import (
	"log"
	"time"

	"github.com/vpsfreecz/vpsadmin-go-client/client"
)

func checkApi(st *Status, checkInterval time.Duration) {
	api := client.New(st.VpsAdmin.Api.Url)

	for {
		publicStatus := api.Node.PublicStatus.Prepare()
		now := time.Now()
		st.VpsAdmin.Api.LastCheck = now

		resp, err := publicStatus.Call()

		if err != nil {
			log.Printf("Unable to check API: %+v", err)
			failApi(st, "", now)
			time.Sleep(checkInterval)
			continue
		} else if !resp.Status {
			log.Printf("Failed to list nodes: %s", resp.Message)
			failApi(st, resp.Message, now)
			time.Sleep(checkInterval)
			continue
		}

		st.VpsAdmin.Api.Status = true
		st.VpsAdmin.Api.Maintenance = false

		for _, node := range resp.Output {
			updateNode(node, st, now)
		}

		time.Sleep(checkInterval)
	}
}

func failApi(st *Status, message string, now time.Time) {
	st.VpsAdmin.Api.Status = false
	st.VpsAdmin.Api.Maintenance = message == "Server under maintenance."

	for _, loc := range st.LocationList {
		for _, node := range loc.NodeList {
			node.ApiStatus = false
			node.LastApiCheck = now
		}
	}
}

func updateNode(apiNode *client.ActionNodePublicStatusOutput, st *Status, now time.Time) {
	stNode := st.GlobalNodeMap[apiNode.Name]
	if stNode == nil {
		log.Printf("Not configured for node %s", apiNode.Name)
		return
	}

	stNode.LastApiCheck = now
	stNode.LocationId = int(apiNode.Location.Id)

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
