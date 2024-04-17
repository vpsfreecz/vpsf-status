package main

import (
	"log"
	"slices"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/vpsfreecz/vpsadmin-go-client/client"
)

var (
	poolStates = []string{"unknown", "online", "degraded", "suspended", "faulted", "error"}
	poolScans  = []string{"none", "scrub", "resilver"}
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

		st.Exporter.vpsAdminStatus.With(prometheus.Labels{"service": "api"}).Set(0)

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

	gauge := st.Exporter.vpsAdminStatus.With(prometheus.Labels{"service": "api"})

	if st.VpsAdmin.Api.Maintenance {
		gauge.Set(1)
	} else {
		gauge.Set(2)
	}
}

func updateNode(apiNode *client.ActionNodePublicStatusOutput, st *Status, now time.Time) {
	stNode := st.GlobalNodeMap[apiNode.Name]
	if stNode == nil {
		log.Printf("Not configured for node %s", apiNode.Name)
		return
	}

	labels := prometheus.Labels{
		"location_id":    strconv.FormatInt(apiNode.Location.Id, 10),
		"location_label": apiNode.Location.Label,
		"node_id":        strconv.Itoa(stNode.Id),
		"node_name":      apiNode.Name,
	}

	nodeStatusGauge := st.Exporter.nodeVpsAdminStatus.With(labels)
	poolStateGauge := st.Exporter.nodePoolState.With(labels)
	poolScanGauge := st.Exporter.nodePoolScan.With(labels)
	poolScanPercentGauge := st.Exporter.nodePoolScanPercent.With(labels)
	poolStatusGauge := st.Exporter.nodePoolStatus.With(labels)

	stNode.LastApiCheck = now
	stNode.LocationId = int(apiNode.Location.Id)
	stNode.OsType = apiNode.HypervisorType

	stNode.PoolState = apiNode.PoolState
	stNode.PoolScan = apiNode.PoolScan
	stNode.PoolScanPercent = apiNode.PoolScanPercent

	if i := slices.Index(poolStates, apiNode.PoolState); i != -1 {
		poolStateGauge.Set(float64(i))
	} else {
		poolStateGauge.Set(0)
	}

	if i := slices.Index(poolScans, apiNode.PoolScan); i != -1 {
		poolScanGauge.Set(float64(i))
	} else {
		poolScanGauge.Set(0)
	}

	poolScanPercentGauge.Set(apiNode.PoolScanPercent)

	if apiNode.MaintenanceLock != "no" {
		stNode.ApiStatus = true
		stNode.ApiMaintenance = true
		stNode.PoolStatus = true
		nodeStatusGauge.Set(0)
		poolStatusGauge.Set(0)
		return
	}

	stNode.ApiMaintenance = false
	stNode.ApiStatus = false
	stNode.PoolStatus = false

	nodeLastReport, err := time.Parse("2006-01-02T15:04:05Z", apiNode.LastReport)
	if err != nil {
		log.Printf("Unable to parse node last_report of %v", apiNode.LastReport)
		nodeStatusGauge.Set(2)
		poolStatusGauge.Set(2)
		return
	}

	nodeDiff := now.Sub(nodeLastReport)
	stNode.ApiStatus = nodeDiff <= (150 * time.Second)

	poolLastReport, err := time.Parse("2006-01-02T15:04:05Z", apiNode.PoolCheckedAt)
	if err != nil {
		log.Printf("Unable to parse node pool_checked_at of %v", apiNode.PoolCheckedAt)
		nodeStatusGauge.Set(2)
		poolStatusGauge.Set(2)
		return
	}

	poolDiff := now.Sub(poolLastReport)
	stNode.PoolStatus = poolDiff <= (150 * time.Second)

	if stNode.ApiMaintenance {
		nodeStatusGauge.Set(1)
		poolStatusGauge.Set(1)
	} else if stNode.ApiStatus {
		nodeStatusGauge.Set(0)
		poolStatusGauge.Set(0)
	} else {
		nodeStatusGauge.Set(2)
		poolStatusGauge.Set(2)
	}
}
