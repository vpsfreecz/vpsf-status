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

type vpsAdminNodeStatusClient interface {
	PublicNodeStatus() (*client.ActionNodePublicStatusResponse, error)
}

type liveVpsAdminNodeStatusClient struct {
	api *client.Client
}

func (c liveVpsAdminNodeStatusClient) PublicNodeStatus() (*client.ActionNodePublicStatusResponse, error) {
	return c.api.Node.PublicStatus.Prepare().Call()
}

func checkApi(st *Status, checkInterval time.Duration) {
	api := liveVpsAdminNodeStatusClient{api: client.New(st.VpsAdmin.Api.Url)}

	for {
		now := time.Now()
		refreshVpsAdminNodesOnce(st, api, now)
		time.Sleep(checkInterval)
	}
}

func refreshVpsAdminNodesOnce(st *Status, api vpsAdminNodeStatusClient, now time.Time) {
	st.VpsAdmin.Api.LastCheck = now

	resp, err := api.PublicNodeStatus()
	if err != nil {
		log.Printf("Unable to check API: %+v", err)
		failApi(st, "", now)
		return
	} else if !resp.Status {
		log.Printf("Failed to list nodes: %s", resp.Message)
		failApi(st, resp.Message, now)
		return
	}

	st.VpsAdmin.Api.Status = true
	st.VpsAdmin.Api.Maintenance = false

	seen := make(map[string]struct{}, len(resp.Output))
	for _, node := range resp.Output {
		if st.GlobalNodeMap[node.Name] != nil {
			seen[node.Name] = struct{}{}
		}
		updateNode(node, st, now)
	}

	for _, loc := range st.LocationList {
		for _, node := range loc.NodeList {
			if _, ok := seen[node.Name]; !ok {
				markNodeMissingFromAPI(st, loc, node, now)
			}
		}
	}

	st.Exporter.vpsAdminStatus.With(prometheus.Labels{"service": "api"}).Set(0)
}

func failApi(st *Status, message string, now time.Time) {
	st.VpsAdmin.Api.Status = false
	st.VpsAdmin.Api.Maintenance = message == "Server under maintenance."

	for _, loc := range st.LocationList {
		for _, node := range loc.NodeList {
			node.ApiStatus = false
			node.ApiMaintenance = st.VpsAdmin.Api.Maintenance
			node.PoolStatus = false
			node.LastApiCheck = now

			status := 2.0
			if st.VpsAdmin.Api.Maintenance {
				status = 1
			}

			labels := nodePrometheusLabels(loc, node)
			st.Exporter.nodeVpsAdminStatus.With(labels).Set(status)
			st.Exporter.nodePoolStatus.With(labels).Set(status)
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

	labels := nodePrometheusLabelsForAPI(st, stNode, apiNode)

	nodeStatusGauge := st.Exporter.nodeVpsAdminStatus.With(labels)
	poolStateGauge := st.Exporter.nodePoolState.With(labels)
	poolScanGauge := st.Exporter.nodePoolScan.With(labels)
	poolScanPercentGauge := st.Exporter.nodePoolScanPercent.With(labels)
	poolStatusGauge := st.Exporter.nodePoolStatus.With(labels)

	stNode.LastApiCheck = now
	if apiNode.Location != nil {
		stNode.LocationId = int(apiNode.Location.Id)
	}
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
		nodeStatusGauge.Set(1)
		poolStatusGauge.Set(1)
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
	} else if stNode.ApiStatus {
		nodeStatusGauge.Set(0)
	} else {
		nodeStatusGauge.Set(2)
	}

	if stNode.ApiMaintenance {
		poolStatusGauge.Set(1)
	} else if stNode.PoolStatus {
		poolStatusGauge.Set(0)
	} else {
		poolStatusGauge.Set(2)
	}
}

func markNodeMissingFromAPI(st *Status, loc *Location, node *Node, now time.Time) {
	node.ApiStatus = false
	node.ApiMaintenance = false
	node.PoolStatus = false
	node.LastApiCheck = now

	labels := nodePrometheusLabels(loc, node)
	st.Exporter.nodeVpsAdminStatus.With(labels).Set(2)
	st.Exporter.nodePoolStatus.With(labels).Set(2)
}

func nodePrometheusLabels(loc *Location, node *Node) prometheus.Labels {
	return prometheus.Labels{
		"location_id":    strconv.Itoa(loc.Id),
		"location_label": loc.Label,
		"node_id":        strconv.Itoa(node.Id),
		"node_name":      node.Name,
	}
}

func nodePrometheusLabelsForAPI(st *Status, node *Node, apiNode *client.ActionNodePublicStatusOutput) prometheus.Labels {
	locationID := node.LocationId
	locationLabel := ""

	if apiNode.Location != nil {
		locationID = int(apiNode.Location.Id)
		locationLabel = apiNode.Location.Label
	} else {
		for _, loc := range st.LocationList {
			if loc.Id == locationID {
				locationLabel = loc.Label
				break
			}
		}
	}

	return prometheus.Labels{
		"location_id":    strconv.Itoa(locationID),
		"location_label": locationLabel,
		"node_id":        strconv.Itoa(node.Id),
		"node_name":      node.Name,
	}
}
