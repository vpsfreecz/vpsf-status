package main

import (
	"fmt"
	"log"
	"time"
)

func recordProbeStatus(st *Status, target ProbeTarget, status string, message string, now time.Time) {
	if st == nil || st.History == nil {
		return
	}

	if err := st.History.RecordProbeStatus(target, status, message, now); err != nil {
		log.Printf("Unable to record probe history: %+v", err)
	}
}

func recordVpsAdminServiceProbe(st *Status, id string, ws *WebService, now time.Time) {
	status, message := webServiceProbeStatus(ws)
	recordProbeStatus(st, ProbeTarget{
		EntityKind:  historyEntityVpsAdmin,
		EntityID:    id,
		EntityLabel: vpsAdminServiceLabel(id),
		Method:      "HTTP",
	}, status, message, now)
}

func recordNodeApiProbe(st *Status, node *Node, now time.Time) {
	status := historyProbeStateDown
	message := "not reporting"

	if node.ApiMaintenance {
		status = historyProbeStateMaintenance
		message = "under maintenance"
	} else if node.ApiStatus {
		status = historyProbeStateOperational
		message = "reporting"
	}

	recordProbeStatus(st, ProbeTarget{
		EntityKind:  historyEntityNode,
		EntityID:    node.Name,
		EntityLabel: node.Name,
		Method:      "vpsAdmin",
	}, status, message, now)
}

func recordNodeStorageProbe(st *Status, node *Node, now time.Time) {
	if !node.IsStorageSupported() {
		return
	}

	status := historyProbeStateError
	message := node.GetStorageStateMessage()
	if node.PoolStatus && node.PoolState == "online" && node.IsStorageScanIssue() {
		message = node.GetStorageScanMessage()
	}

	if node.ApiMaintenance {
		status = historyProbeStateMaintenance
		message = "under maintenance"
	} else if node.IsStorageOperational() {
		status = historyProbeStateOperational
		message = "online"
	} else if node.IsStorageHardFailure() {
		status = historyProbeStateDown
	} else if node.IsStorageDegraded() {
		status = historyProbeStateDegraded
		if node.IsStorageScrubIssue() || node.IsStorageResilverIssue() {
			message = node.GetStorageScanMessage()
		}
	}

	recordProbeStatus(st, ProbeTarget{
		EntityKind:  historyEntityNode,
		EntityID:    node.Name,
		EntityLabel: node.Name,
		Method:      "Storage",
	}, status, message, now)
}

func recordNodeProbes(st *Status, node *Node, now time.Time) {
	recordNodeApiProbe(st, node, now)
	recordNodeStorageProbe(st, node, now)
}

func webServiceProbeStatus(ws *WebService) (string, string) {
	if ws.Status {
		return historyProbeStateOperational, "HTTP 200"
	}

	if ws.Maintenance {
		return historyProbeStateMaintenance, "HTTP 503"
	}

	if ws.StatusCode == 0 {
		return historyProbeStateError, "check failed"
	}

	return historyProbeStateDown, fmt.Sprintf("HTTP %d", ws.StatusCode)
}

func pingProbeStatus(pc *PingCheck) (string, string) {
	if pc.IsUp() {
		return historyProbeStateOperational, "responding"
	}

	if pc.IsWarning() {
		return historyProbeStateDegraded, fmt.Sprintf("%.0f%% packet loss", pc.PacketLoss)
	}

	return historyProbeStateDown, "not responding"
}

func vpsAdminServiceLabel(id string) string {
	switch id {
	case "api":
		return "vpsAdmin API"
	case "webui":
		return "vpsAdmin web UI"
	case "console":
		return "Remote Console"
	default:
		return id
	}
}
