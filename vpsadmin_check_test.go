package main

import (
	"errors"
	"testing"
	"time"

	"github.com/vpsfreecz/vpsadmin-go-client/client"
)

type fakeNodeStatusClient struct {
	resp *client.ActionNodePublicStatusResponse
	err  error
}

func (f fakeNodeStatusClient) PublicNodeStatus() (*client.ActionNodePublicStatusResponse, error) {
	return f.resp, f.err
}

func TestRefreshVpsAdminNodesOnceUpdatesConfiguredNodes(t *testing.T) {
	app, st, _ := newTestApplication(t)
	setOperationalFixture(st)

	resp := nodeStatusResponse(true, "", []*client.ActionNodePublicStatusOutput{
		apiNode("node1.prg", fixedNow.Add(-30*time.Second), fixedNow.Add(-30*time.Second), "online", "none", 0, "no"),
		apiNode("node2.prg", fixedNow.Add(-30*time.Second), fixedNow.Add(-30*time.Second), "degraded", "resilver", 42.5, "no"),
		apiNode("unknown.prg", fixedNow.Add(-30*time.Second), fixedNow.Add(-30*time.Second), "online", "none", 0, "no"),
	})

	refreshVpsAdminNodesOnce(st, fakeNodeStatusClient{resp: resp}, fixedNow)

	if !st.VpsAdmin.Api.Status || st.VpsAdmin.Api.Maintenance {
		t.Fatalf("vpsAdmin API = %+v", st.VpsAdmin.Api)
	}
	if _, ok := st.GlobalNodeMap["unknown.prg"]; ok {
		t.Fatal("unknown node was added to GlobalNodeMap")
	}

	node1 := st.GlobalNodeMap["node1.prg"]
	if !node1.ApiStatus || !node1.PoolStatus || node1.PoolState != "online" || node1.PoolScan != "none" {
		t.Fatalf("node1 = %+v", node1)
	}
	node2 := st.GlobalNodeMap["node2.prg"]
	if !node2.ApiStatus || !node2.PoolStatus || node2.PoolState != "degraded" || node2.PoolScan != "resilver" || node2.PoolScanPercent != 42.5 {
		t.Fatalf("node2 = %+v", node2)
	}
	if !node2.IsDegraded() {
		t.Fatalf("node2 should be degraded: %+v", node2)
	}

	families := scrapeMetrics(t, app)
	requireMetricValue(t, families["vpsfstatus_vpsadmin_status"], map[string]string{"service": "api"}, 0)
	labels := map[string]string{"location_id": "3", "location_label": "Praha", "node_id": "102", "node_name": "node2.prg"}
	requireMetricValue(t, families["vpsfstatus_node_vpsadmin_status"], labels, 0)
	requireMetricValue(t, families["vpsfstatus_node_pool_state"], labels, 2)
	requireMetricValue(t, families["vpsfstatus_node_pool_scan"], labels, 2)
	requireMetricValue(t, families["vpsfstatus_node_pool_scan_percent"], labels, 42.5)
	requireMetricValue(t, families["vpsfstatus_node_pool_status"], labels, 0)
}

func TestRefreshVpsAdminNodesOnceHandlesStaleReports(t *testing.T) {
	app, st, _ := newTestApplication(t)
	setOperationalFixture(st)

	resp := nodeStatusResponse(true, "", []*client.ActionNodePublicStatusOutput{
		apiNode("node1.prg", fixedNow.Add(-3*time.Minute), fixedNow.Add(-3*time.Minute), "online", "none", 0, "no"),
	})

	refreshVpsAdminNodesOnce(st, fakeNodeStatusClient{resp: resp}, fixedNow)

	node := st.GlobalNodeMap["node1.prg"]
	if node.ApiStatus || node.PoolStatus {
		t.Fatalf("stale node should be down: %+v", node)
	}

	families := scrapeMetrics(t, app)
	labels := map[string]string{"location_id": "3", "location_label": "Praha", "node_id": "101", "node_name": "node1.prg"}
	requireMetricValue(t, families["vpsfstatus_node_vpsadmin_status"], labels, 2)
	requireMetricValue(t, families["vpsfstatus_node_pool_status"], labels, 2)
}

func TestRefreshVpsAdminNodesOnceHandlesFreshNodeWithStalePool(t *testing.T) {
	app, st, _ := newTestApplication(t)
	setOperationalFixture(st)

	resp := nodeStatusResponse(true, "", []*client.ActionNodePublicStatusOutput{
		apiNode("node1.prg", fixedNow.Add(-30*time.Second), fixedNow.Add(-3*time.Minute), "online", "none", 0, "no"),
	})

	refreshVpsAdminNodesOnce(st, fakeNodeStatusClient{resp: resp}, fixedNow)

	node := st.GlobalNodeMap["node1.prg"]
	if !node.ApiStatus || node.PoolStatus {
		t.Fatalf("node should have fresh vpsAdmin status and stale pool status: %+v", node)
	}

	families := scrapeMetrics(t, app)
	labels := map[string]string{"location_id": "3", "location_label": "Praha", "node_id": "101", "node_name": "node1.prg"}
	requireMetricValue(t, families["vpsfstatus_node_vpsadmin_status"], labels, 0)
	requireMetricValue(t, families["vpsfstatus_node_pool_status"], labels, 2)
}

func TestRefreshVpsAdminNodesOnceHandlesMaintenanceLock(t *testing.T) {
	app, st, _ := newTestApplication(t)
	setOperationalFixture(st)

	resp := nodeStatusResponse(true, "", []*client.ActionNodePublicStatusOutput{
		apiNode("node1.prg", fixedNow.Add(-3*time.Minute), fixedNow.Add(-3*time.Minute), "degraded", "scrub", 10, "yes"),
	})

	refreshVpsAdminNodesOnce(st, fakeNodeStatusClient{resp: resp}, fixedNow)

	node := st.GlobalNodeMap["node1.prg"]
	if !node.ApiStatus || !node.ApiMaintenance || !node.PoolStatus {
		t.Fatalf("node should be treated as under maintenance: %+v", node)
	}

	families := scrapeMetrics(t, app)
	labels := map[string]string{"location_id": "3", "location_label": "Praha", "node_id": "101", "node_name": "node1.prg"}
	requireMetricValue(t, families["vpsfstatus_node_vpsadmin_status"], labels, 1)
	requireMetricValue(t, families["vpsfstatus_node_pool_status"], labels, 1)
}

func TestRefreshVpsAdminNodesOnceMarksMissingConfiguredNodesDown(t *testing.T) {
	app, st, _ := newTestApplication(t)
	setOperationalFixture(st)

	setNodeState(st, st.LocationMap["Praha"], st.GlobalNodeMap["node2.prg"], true, false, "vpsadminos", true, "degraded", "resilver", 42.5, 0)
	resp := nodeStatusResponse(true, "", []*client.ActionNodePublicStatusOutput{
		apiNode("node1.prg", fixedNow.Add(-30*time.Second), fixedNow.Add(-30*time.Second), "online", "none", 0, "no"),
	})

	refreshVpsAdminNodesOnce(st, fakeNodeStatusClient{resp: resp}, fixedNow)

	missing := st.GlobalNodeMap["node2.prg"]
	if missing.ApiStatus || missing.ApiMaintenance || missing.PoolStatus {
		t.Fatalf("missing configured node should be marked down: %+v", missing)
	}
	if missing.PoolState != "unknown" || missing.PoolScan != "none" || missing.PoolScanPercent != 0 {
		t.Fatalf("missing configured node should have unknown pool metrics: %+v", missing)
	}

	families := scrapeMetrics(t, app)
	labels := map[string]string{"location_id": "3", "location_label": "Praha", "node_id": "102", "node_name": "node2.prg"}
	requireMetricValue(t, families["vpsfstatus_node_vpsadmin_status"], labels, 2)
	requireMetricValue(t, families["vpsfstatus_node_pool_status"], labels, 2)
	requireMetricValue(t, families["vpsfstatus_node_pool_state"], labels, 0)
	requireMetricValue(t, families["vpsfstatus_node_pool_scan"], labels, 0)
	requireMetricValue(t, families["vpsfstatus_node_pool_scan_percent"], labels, 0)
}

func TestRefreshVpsAdminNodesOnceHandlesAPIFailures(t *testing.T) {
	tests := []struct {
		name            string
		resp            *client.ActionNodePublicStatusResponse
		err             error
		wantMaintenance bool
		wantGauge       float64
	}{
		{
			name:      "request error",
			err:       errors.New("api unavailable"),
			wantGauge: 2,
		},
		{
			name:            "maintenance message",
			resp:            nodeStatusResponse(false, "Server under maintenance.", nil),
			wantMaintenance: true,
			wantGauge:       1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, st, _ := newTestApplication(t)
			setOperationalFixture(st)
			setNodeState(st, st.LocationMap["Praha"], st.GlobalNodeMap["node1.prg"], true, false, "vpsadminos", true, "faulted", "resilver", 42.5, 0)

			refreshVpsAdminNodesOnce(st, fakeNodeStatusClient{resp: tt.resp, err: tt.err}, fixedNow)

			if st.VpsAdmin.Api.Status || st.VpsAdmin.Api.Maintenance != tt.wantMaintenance {
				t.Fatalf("vpsAdmin API = %+v", st.VpsAdmin.Api)
			}
			for _, loc := range st.LocationList {
				for _, node := range loc.NodeList {
					if node.ApiStatus || node.PoolStatus {
						t.Fatalf("node should be down after API failure: %+v", node)
					}
				}
			}
			node := st.GlobalNodeMap["node1.prg"]
			if node.PoolState != "unknown" || node.PoolScan != "none" || node.PoolScanPercent != 0 {
				t.Fatalf("node should have unknown pool metrics after API failure: %+v", node)
			}

			families := scrapeMetrics(t, app)
			labels := map[string]string{"location_id": "3", "location_label": "Praha", "node_id": "101", "node_name": "node1.prg"}
			requireMetricValue(t, families["vpsfstatus_vpsadmin_status"], map[string]string{"service": "api"}, tt.wantGauge)
			requireMetricValue(t, families["vpsfstatus_node_vpsadmin_status"], labels, tt.wantGauge)
			requireMetricValue(t, families["vpsfstatus_node_pool_status"], labels, tt.wantGauge)
			requireMetricValue(t, families["vpsfstatus_node_pool_state"], labels, 0)
			requireMetricValue(t, families["vpsfstatus_node_pool_scan"], labels, 0)
			requireMetricValue(t, families["vpsfstatus_node_pool_scan_percent"], labels, 0)
		})
	}
}

func nodeStatusResponse(status bool, message string, output []*client.ActionNodePublicStatusOutput) *client.ActionNodePublicStatusResponse {
	return &client.ActionNodePublicStatusResponse{
		Envelope: &client.Envelope{Status: status, Message: message},
		Output:   output,
	}
}

func apiNode(name string, lastReport time.Time, poolCheckedAt time.Time, poolState string, poolScan string, poolScanPercent float64, maintenanceLock string) *client.ActionNodePublicStatusOutput {
	return &client.ActionNodePublicStatusOutput{
		Name:            name,
		Location:        &client.ActionLocationShowOutput{Id: 3, Label: "Praha"},
		HypervisorType:  "vpsadminos",
		LastReport:      apiTimestamp(lastReport),
		PoolCheckedAt:   apiTimestamp(poolCheckedAt),
		PoolState:       poolState,
		PoolScan:        poolScan,
		PoolScanPercent: poolScanPercent,
		MaintenanceLock: maintenanceLock,
	}
}

func apiTimestamp(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05Z")
}
