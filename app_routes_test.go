package main

import (
	"bytes"
	stdjson "encoding/json"
	"net/http"
	"testing"

	statusjson "github.com/vpsfreecz/vpsf-status/json"
)

func TestRoutesServeIndexLoadingState(t *testing.T) {
	app, st, cfg := newTestApplication(t)
	writeNotice(t, cfg, "<p>Checks are starting.</p>")
	st.Exporter.notice.Set(1)

	rr := getThroughRoutes(t, app, "/")
	requireStatus(t, rr, http.StatusOK)

	if got := rr.Header().Get("Cache-Control"); got != "max-age=1" {
		t.Fatalf("Cache-Control = %q, want max-age=1", got)
	}

	requireContains(
		t,
		rr.Body.String(),
		"Initializing...",
		"vpsFree.cz Status is initializing",
		"Rendered at: Sat May  2 10:30:00 UTC 2026",
		"Checks are starting.",
		"Updated at "+fixedNoticeTime.Local().Format("Mon Jan _2 15:04:05 MST 2006"),
	)
}

func TestRoutesServeIndexOperationalState(t *testing.T) {
	app, st, _ := newTestApplication(t)
	setOperationalFixture(st)

	rr := getThroughRoutes(t, app, "/")
	requireStatus(t, rr, http.StatusOK)

	body := rr.Body.String()
	requireContains(
		t,
		body,
		"No issues reported.",
		"https://vpsadmin.vpsfree.cz/?page=outage&action=list",
		"vpsAdmin 3/3",
		"Web Services 3/3",
		"Praha 3/3",
		"Nodes 2/2",
		"DNS Resolvers 1/1",
		"Brno 1/1",
		"Services 3/3",
		"Name Servers 1/1",
		"node1.prg",
		"node2.prg",
		"resolver-prg",
		"vpsfree.cz",
		"ns1.vpsfree.cz",
		`aria-label="Operational"`,
		`aria-label="Responding"`,
	)
	requireNotContains(t, body, "Reported Maintenances", "Unable to fetch outage reports")
}

func TestRoutesServeIndexPartialDownState(t *testing.T) {
	app, st, _ := newTestApplication(t)
	setOperationalFixture(st)

	setWebServiceState(st, st.Services.Web[1], false, false, http.StatusInternalServerError)
	node := st.GlobalNodeMap["node2.prg"]
	setNodeState(st, st.LocationMap["Praha"], node, false, false, "vpsadminos", false, "online", "none", 0, 100)

	rr := getThroughRoutes(t, app, "/")
	requireStatus(t, rr, http.StatusOK)

	requireContains(
		t,
		rr.Body.String(),
		"Praha 2/3",
		"Nodes 1/2",
		"Services 2/3",
		"Web Services 1/2",
		"kb.vpsfree.cz",
		`aria-label="Down"`,
	)
}

func TestRoutesServeIndexMaintenanceAndDegradedState(t *testing.T) {
	app, st, cfg := newTestApplication(t)
	setOperationalFixture(st)
	writeNotice(t, cfg, "<p>Maintenance notice</p>")
	st.Exporter.notice.Set(1)
	addOutageFixture(st)

	setVpsAdminService(st, "webui", st.VpsAdmin.Webui, false, true, http.StatusServiceUnavailable)
	node := st.GlobalNodeMap["node2.prg"]
	setNodeState(st, st.LocationMap["Praha"], node, true, false, "vpsadminos", true, "degraded", "resilver", 42.5, 40)

	rr := getThroughRoutes(t, app, "/")
	requireStatus(t, rr, http.StatusOK)

	requireContains(
		t,
		rr.Body.String(),
		"Maintenance notice",
		"Reported",
		"Maintenances",
		"Resolved",
		"Outages",
		"Router replacement",
		"Power failure",
		"node1.prg",
		"https://vpsadmin.vpsfree.cz/?page=outage&action=show&id=1001",
		"vpsAdmin 3/3",
		"Praha 3/3",
		"Nodes 2/2",
		`aria-label="Under maintenance"`,
		`aria-label="Degraded"`,
		`Storage is being resilvered to replace disks, 42.5 % done`,
	)
}

func TestRoutesServeIndexAllDownState(t *testing.T) {
	app, st, _ := newTestApplication(t)
	setOperationalFixture(st)

	setVpsAdminService(st, "api", st.VpsAdmin.Api, false, false, http.StatusInternalServerError)
	setVpsAdminService(st, "webui", st.VpsAdmin.Webui, false, false, http.StatusInternalServerError)
	setVpsAdminService(st, "console", st.VpsAdmin.Console, false, false, http.StatusInternalServerError)
	for _, loc := range st.LocationList {
		for _, node := range loc.NodeList {
			setNodeState(st, loc, node, false, false, "vpsadminos", false, "unknown", "none", 0, 100)
		}
		for _, dns := range loc.DnsResolverList {
			setResolverState(st.Exporter.dnsResolverPing, st.Exporter.dnsResolverLookup, dns, 100, false)
		}
	}
	for _, ws := range st.Services.Web {
		setWebServiceState(st, ws, false, false, http.StatusInternalServerError)
	}
	for _, ns := range st.Services.NameServer {
		setResolverState(st.Exporter.nameServerPing, st.Exporter.nameServerLookup, ns, 100, false)
	}
	st.OutageReports.Status = false

	rr := getThroughRoutes(t, app, "/")
	requireStatus(t, rr, http.StatusOK)

	requireContains(
		t,
		rr.Body.String(),
		"Unable to fetch outage reports from vpsAdmin.",
		"vpsAdmin 0/3",
		"Praha 0/3",
		"Brno 0/1",
		"Services 0/3",
		`aria-label="Down"`,
		`aria-label="Error"`,
	)
}

func TestRoutesServeAboutAndStaticAssets(t *testing.T) {
	app, st, _ := newTestApplication(t)
	setOperationalFixture(st)

	about := getThroughRoutes(t, app, "/about")
	requireStatus(t, about, http.StatusOK)
	requireContains(
		t,
		about.Body.String(),
		"About vpsFree.cz Status",
		"Back to Status",
		`href="/json"`,
		`href="/metrics"`,
		`href="/about"`,
		"https://github.com/vpsfreecz/vpsf-status",
	)

	static := getThroughRoutes(t, app, "/static/favicon.png")
	requireStatus(t, static, http.StatusOK)
	if static.Body.Len() == 0 {
		t.Fatal("static favicon response body is empty")
	}
}

func TestRoutesServeJSONContract(t *testing.T) {
	app, st, cfg := newTestApplication(t)
	setOperationalFixture(st)
	writeNotice(t, cfg, "<p>Maintenance notice</p>")
	addOutageFixture(st)

	setVpsAdminService(st, "webui", st.VpsAdmin.Webui, false, true, http.StatusServiceUnavailable)
	setVpsAdminService(st, "console", st.VpsAdmin.Console, false, false, http.StatusInternalServerError)
	node := st.GlobalNodeMap["node2.prg"]
	setNodeState(st, st.LocationMap["Praha"], node, true, false, "vpsadminos", true, "degraded", "resilver", 42.5, 40)
	setWebServiceState(st, st.Services.Web[1], false, false, http.StatusInternalServerError)
	setResolverState(st.Exporter.nameServerPing, st.Exporter.nameServerLookup, st.Services.NameServer[0], 100, true)

	rr := getThroughRoutes(t, app, "/json")
	requireStatus(t, rr, http.StatusOK)

	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	if got := rr.Header().Get("Cache-Control"); got != "max-age=1" {
		t.Fatalf("Cache-Control = %q, want max-age=1", got)
	}

	rawBytes := rr.Body.Bytes()
	var raw map[string]any
	if err := stdjson.Unmarshal(rawBytes, &raw); err != nil {
		t.Fatalf("decode raw JSON: %v", err)
	}
	requireJSONContractKeys(t, raw)

	var body statusjson.Status
	if err := stdjson.NewDecoder(bytes.NewReader(rawBytes)).Decode(&body); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}

	if !body.GeneratedAt.Equal(fixedNow) {
		t.Fatalf("generated_at = %s, want %s", body.GeneratedAt, fixedNow)
	}
	if !body.Notice.Any || body.Notice.Text != "<p>Maintenance notice</p>" || !body.Notice.UpdatedAt.Equal(fixedNoticeTime) {
		t.Fatalf("notice = %+v", body.Notice)
	}

	if body.VpsAdmin.Api.Status != "operational" {
		t.Fatalf("vpsadmin.api.status = %q", body.VpsAdmin.Api.Status)
	}
	if body.VpsAdmin.Webui.Status != "maintenance" {
		t.Fatalf("vpsadmin.webui.status = %q", body.VpsAdmin.Webui.Status)
	}
	if body.VpsAdmin.Console.Status != "down" {
		t.Fatalf("vpsadmin.console.status = %q", body.VpsAdmin.Console.Status)
	}

	if !body.OutageReports.Status || len(body.OutageReports.Announced) != 1 || len(body.OutageReports.Recent) != 1 {
		t.Fatalf("outage_reports = %+v", body.OutageReports)
	}
	if got := body.OutageReports.Announced[0]; got.Id != 1001 || got.Type != "maintenance" || got.State != "announced" || got.Entities[0].Label != "node1.prg" {
		t.Fatalf("announced outage = %+v", got)
	}
	if got := body.OutageReports.Recent[0]; got.Id != 1002 || got.Type != "outage" || got.State != "resolved" || got.Entities[0].Label != "Praha" {
		t.Fatalf("recent outage = %+v", got)
	}

	if len(body.Locations) != 2 || body.Locations[0].Label != "Praha" || len(body.Locations[0].Nodes) != 2 {
		t.Fatalf("locations = %+v", body.Locations)
	}
	if got := body.Locations[0].Nodes[1]; got.Name != "node2.prg" || got.Ping != "degraded" || got.PoolState != "degraded" || got.PoolScan != "resilver" || got.PoolScanPercent != 42.5 || !got.PoolStatus {
		t.Fatalf("node2 JSON = %+v", got)
	}
	if got := body.Locations[0].DnsResolvers[0]; got.Name != "resolver-prg" || got.Ping != "responding" || !got.Lookup {
		t.Fatalf("dns resolver JSON = %+v", got)
	}

	if len(body.WebServices) != 2 || body.WebServices[0].Status != "operational" || body.WebServices[1].Status != "down" {
		t.Fatalf("web_services = %+v", body.WebServices)
	}
	if len(body.NameServers) != 1 || body.NameServers[0].Name != "ns1.vpsfree.cz" || body.NameServers[0].Ping != "down" || !body.NameServers[0].Lookup {
		t.Fatalf("nameservers = %+v", body.NameServers)
	}
}

func requireJSONContractKeys(t *testing.T, raw map[string]any) {
	t.Helper()

	requireMapKeys(t, raw, "generated_at", "locations", "nameservers", "notice", "outage_reports", "vpsadmin", "web_services")

	notice := requireMapValue(t, raw, "notice")
	requireMapKeys(t, notice, "any", "text", "updated_at")

	vpsadmin := requireMapValue(t, raw, "vpsadmin")
	requireMapKeys(t, vpsadmin, "api", "console", "webui")
	requireMapKeys(t, requireMapValue(t, vpsadmin, "api"), "description", "label", "status", "url")
	requireMapKeys(t, requireMapValue(t, vpsadmin, "console"), "description", "label", "status", "url")
	requireMapKeys(t, requireMapValue(t, vpsadmin, "webui"), "description", "label", "status", "url")

	outages := requireMapValue(t, raw, "outage_reports")
	requireMapKeys(t, outages, "announced", "recent", "status")
	requireOutageJSONKeys(t, requireSliceMap(t, requireSliceValue(t, outages, "announced"), 0))
	requireOutageJSONKeys(t, requireSliceMap(t, requireSliceValue(t, outages, "recent"), 0))

	locations := requireSliceValue(t, raw, "locations")
	location := requireSliceMap(t, locations, 0)
	requireMapKeys(t, location, "dns_resolvers", "id", "label", "nodes")
	requireMapKeys(t, requireSliceMap(t, requireSliceValue(t, location, "nodes"), 0), "id", "location_id", "maintenance", "name", "os_type", "ping", "pool_scan", "pool_scan_percent", "pool_state", "pool_status", "vpsadmin")
	requireMapKeys(t, requireSliceMap(t, requireSliceValue(t, location, "dns_resolvers"), 0), "lookup", "name", "ping")

	requireMapKeys(t, requireSliceMap(t, requireSliceValue(t, raw, "web_services"), 0), "description", "label", "status", "url")
	requireMapKeys(t, requireSliceMap(t, requireSliceValue(t, raw, "nameservers"), 0), "lookup", "name", "ping")
}

func requireOutageJSONKeys(t *testing.T, outage map[string]any) {
	t.Helper()

	requireMapKeys(t, outage, "begins_at", "cs_description", "cs_summary", "duration", "en_description", "en_summary", "entities", "id", "impact", "state", "type")
	requireMapKeys(t, requireSliceMap(t, requireSliceValue(t, outage, "entities"), 0), "id", "label", "name")
}
