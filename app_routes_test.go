package main

import (
	"bytes"
	stdjson "encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

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
		`href="/entity?kind=node&amp;id=node1.prg"`,
		"node2.prg",
		"resolver-prg",
		"vpsfree.cz",
		"ns1.vpsfree.cz",
		"history-day-ok",
		`aria-label="vpsAdmin history"`,
		`aria-label="Praha history"`,
		`aria-label="Brno history"`,
		`aria-label="Services history"`,
		"history-bar-split",
		"history-lane",
		`aria-label="Operational"`,
		`aria-label="Responding"`,
	)
	requireNotContains(t, body, "Reported Maintenances", "Unable to fetch outage reports", "Last 90 days", "Overall status history")
}

func TestRoutesServeEntityDetail(t *testing.T) {
	app, st, _ := newTestApplication(t)
	setOperationalFixture(st)

	target := ProbeTarget{
		EntityKind:  historyEntityNode,
		EntityID:    "node1.prg",
		EntityLabel: "node1.prg",
		Method:      "Ping",
	}
	if err := st.History.RecordProbeStatus(target, historyProbeStateDown, "not responding", fixedNow.Add(-10*time.Minute)); err != nil {
		t.Fatalf("record down status: %v", err)
	}
	if err := st.History.RecordProbeStatus(target, historyProbeStateOperational, "responding", fixedNow.Add(-2*time.Minute)); err != nil {
		t.Fatalf("record recovery: %v", err)
	}

	rr := getThroughRoutes(t, app, "/entity?kind=node&id=node1.prg")
	requireStatus(t, rr, http.StatusOK)

	requireContains(
		t,
		rr.Body.String(),
		"node1.prg",
		`<a class="navbar-link" href="/">Back to Status</a>`,
		"Availability",
		"30 days",
		"90 days",
		"180 days",
		"1 year",
		"Reported",
		"Probe",
		"100.000%",
		"n/a",
		"Probe log",
		"Ping",
		"Down",
		"not responding",
		"Probe: node1.prg Ping not responding",
		"history-day-maintenance",
	)

	body := rr.Body.String()
	requireNotContains(t, body, "Rendered at:", `<p><a href="/">Status</a></p>`)
	if strings.Index(body, "Availability") > strings.Index(body, "Probe log") {
		t.Fatalf("availability should be rendered above probe log")
	}
}

func TestRoutesServeEntityDetailReportedAvailabilityWithoutProbeLogRows(t *testing.T) {
	app, st, _ := newTestApplication(t)
	setOperationalFixture(st)

	windowStart := fixedNow.AddDate(0, 0, -30)
	if err := st.History.ReplaceOutages([]*OutageReport{
		availabilityTestOutage(7001, windowStart.Add(24*time.Hour), 24*time.Hour, []OutageEntity{
			{Name: "Node", Id: 101, Label: "Node node1.prg"},
		}),
	}, fixedNow); err != nil {
		t.Fatalf("replace outages: %v", err)
	}

	rr := getThroughRoutes(t, app, "/entity?kind=node&id=node1.prg")
	requireStatus(t, rr, http.StatusOK)

	requireContains(
		t,
		rr.Body.String(),
		"Availability",
		"Reported",
		"Probe",
		"96.667%",
		"n/a",
		"Probe log",
		"No probe changes recorded.",
	)
}

func TestRoutesServeEntityDetailHidesUnsupportedReportedAvailability(t *testing.T) {
	app, st, _ := newTestApplication(t)
	setOperationalFixture(st)

	if err := st.History.RecordProbeStatus(ProbeTarget{
		EntityKind:  historyEntityWebService,
		EntityID:    "vpsfree.cz",
		EntityLabel: "vpsfree.cz",
		Method:      "HTTP",
	}, historyProbeStateOperational, "HTTP 200", fixedNow.AddDate(-1, 0, 0).Add(-time.Hour)); err != nil {
		t.Fatalf("record probe status: %v", err)
	}

	rr := getThroughRoutes(t, app, "/entity?kind=web_service&id=vpsfree.cz")
	requireStatus(t, rr, http.StatusOK)

	body := rr.Body.String()
	requireContains(
		t,
		body,
		"Availability",
		"Probe",
		"30 days",
		"100.000%",
	)
	requireNotContains(t, body, "Reported")
}

func TestRoutesServeHistoryPopoverWithOutageLinks(t *testing.T) {
	app, st, _ := newTestApplication(t)
	setOperationalFixture(st)

	if err := st.History.ReplaceOutages([]*OutageReport{
		{
			Id:        2001,
			BeginsAt:  fixedNow.Add(-24 * time.Hour),
			Duration:  30 * time.Minute,
			Type:      "outage",
			State:     "resolved",
			EnSummary: "Power failure",
			AffectedEntities: []OutageEntity{
				{Name: "Node", Id: 101, Label: "Node node1.prg"},
			},
		},
	}, fixedNow); err != nil {
		t.Fatalf("replace outages: %v", err)
	}

	rr := getThroughRoutes(t, app, "/")
	requireStatus(t, rr, http.StatusOK)

	requireContains(
		t,
		rr.Body.String(),
		"history-popover",
		"Outage: Power failure",
		`href="https://vpsadmin.vpsfree.cz/?page=outage&amp;action=show&amp;id=2001"`,
		`target="_blank"`,
	)
}

func TestRoutesServeEntityDetailNotFound(t *testing.T) {
	app, st, _ := newTestApplication(t)
	setOperationalFixture(st)

	rr := getThroughRoutes(t, app, "/entity?kind=node&id=missing")
	requireStatus(t, rr, http.StatusNotFound)
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

func TestRoutesServeIndexNoticeSuppressesNoIssues(t *testing.T) {
	app, st, cfg := newTestApplication(t)
	setOperationalFixture(st)
	writeNotice(t, cfg, "<p>Maintenance notice</p>")

	rr := getThroughRoutes(t, app, "/")
	requireStatus(t, rr, http.StatusOK)

	body := rr.Body.String()
	requireContains(t, body, "Maintenance notice")
	requireNotContains(t, body, "No issues reported.")
}

func TestRoutesServeIndexWebMaintenanceAndStorageBranches(t *testing.T) {
	app, st, _ := newTestApplication(t)
	setOperationalFixture(st)

	setWebServiceState(st, st.Services.Web[1], false, true, http.StatusServiceUnavailable)
	node1 := st.GlobalNodeMap["node1.prg"]
	setNodeState(st, st.LocationMap["Praha"], node1, true, false, "openvz", false, "unknown", "none", 0, 0)
	node2 := st.GlobalNodeMap["node2.prg"]
	setNodeState(st, st.LocationMap["Praha"], node2, true, false, "vpsadminos", true, "suspended", "scrub", 15, 0)

	rr := getThroughRoutes(t, app, "/")
	requireStatus(t, rr, http.StatusOK)

	requireContains(
		t,
		rr.Body.String(),
		"Praha 3/3",
		"Nodes 2/2",
		"Services 3/3",
		"Web Services 2/2",
		`aria-label="Under maintenance"`,
		`aria-label="Not supported"`,
		"Storage not operational",
		"Storage is being scrubbed to check data integrity, 15.0 % done",
	)
}

func TestRoutesServeIndexTreatsOnlineScrubAsOperational(t *testing.T) {
	app, st, _ := newTestApplication(t)
	setOperationalFixture(st)

	node := st.GlobalNodeMap["node2.prg"]
	setNodeState(st, st.LocationMap["Praha"], node, true, false, "vpsadminos", true, "online", "scrub", 15, 0)

	rr := getThroughRoutes(t, app, "/")
	requireStatus(t, rr, http.StatusOK)

	body := rr.Body.String()
	requireContains(
		t,
		body,
		"Praha 3/3",
		"Nodes 2/2",
		"Storage is being scrubbed to check data integrity, 15.0 % done",
	)

	prahaIndex := strings.Index(body, "Praha 3/3")
	if prahaIndex == -1 {
		t.Fatalf("Praha heading not found in body")
	}
	prahaHeadingStart := strings.LastIndex(body[:prahaIndex], "<h2")
	prahaHeadingEnd := strings.Index(body[prahaIndex:], "</h2>")
	if prahaHeadingStart == -1 || prahaHeadingEnd == -1 {
		t.Fatalf("Praha heading boundaries not found in body")
	}
	prahaHeading := body[prahaHeadingStart : prahaIndex+prahaHeadingEnd]
	if !strings.Contains(prahaHeading, `class="text-success"`) {
		t.Fatalf("Praha heading = %q, want success", prahaHeading)
	}

	nodesIndex := strings.Index(body, "Nodes 2/2")
	if nodesIndex == -1 {
		t.Fatalf("Nodes heading not found in body")
	}
	nodesButtonStart := strings.LastIndex(body[:nodesIndex], "<button")
	nodesButtonEnd := strings.Index(body[nodesIndex:], "</button>")
	if nodesButtonStart == -1 || nodesButtonEnd == -1 {
		t.Fatalf("Nodes button boundaries not found in body")
	}
	nodesButton := body[nodesButtonStart : nodesIndex+nodesButtonEnd]
	if !strings.Contains(nodesButton, `text-success`) {
		t.Fatalf("Nodes button = %q, want success", nodesButton)
	}

	nodeIndex := strings.Index(body, `href="/entity?kind=node&amp;id=node2.prg">node2.prg</a>`)
	if nodeIndex == -1 {
		t.Fatalf("node2.prg row not found in body")
	}
	nodeRowStart := strings.LastIndex(body[:nodeIndex], "<tr")
	nodeRowEnd := strings.Index(body[nodeIndex:], "</tr>")
	if nodeRowStart == -1 || nodeRowEnd == -1 {
		t.Fatalf("node2.prg row boundaries not found in body")
	}
	nodeRow := body[nodeRowStart : nodeIndex+nodeRowEnd]
	if !strings.Contains(nodeRow, `class="table-success"`) {
		t.Fatalf("node2.prg row = %q, want success", nodeRow)
	}
	if strings.Contains(nodeRow, `text-warning`) {
		t.Fatalf("node2.prg row = %q, should not contain a warning style", nodeRow)
	}
}

func TestRoutesServeIndexMixedOutageHeadings(t *testing.T) {
	app, st, _ := newTestApplication(t)
	setOperationalFixture(st)

	st.OutageReports = &OutageReports{
		Status:               true,
		AnyActive:            true,
		AnyActiveMaintenance: true,
		AnyActiveOutage:      true,
		AnyRecent:            true,
		AnyRecentMaintenance: true,
		AnyRecentOutage:      true,
		ActiveList: []*OutageReport{
			{
				Id:        1001,
				BeginsAt:  fixedNow.Add(2 * time.Hour),
				Duration:  90 * time.Minute,
				Type:      "maintenance",
				State:     "announced",
				Impact:    "partial",
				EnSummary: "Router replacement",
				AffectedEntities: []OutageEntity{
					{Name: "node", Id: 101, Label: "node1.prg"},
				},
			},
			{
				Id:        1002,
				BeginsAt:  fixedNow.Add(3 * time.Hour),
				Duration:  45 * time.Minute,
				Type:      "outage",
				State:     "announced",
				Impact:    "full",
				EnSummary: "Switch down",
				AffectedEntities: []OutageEntity{
					{Name: "location", Id: 3, Label: "Praha"},
				},
			},
		},
		RecentList: []*OutageReport{
			{
				Id:        1003,
				BeginsAt:  fixedNow.Add(-2 * time.Hour),
				Duration:  30 * time.Minute,
				Type:      "maintenance",
				State:     "resolved",
				Impact:    "partial",
				EnSummary: "Old maintenance",
				AffectedEntities: []OutageEntity{
					{Name: "node", Id: 102, Label: "node2.prg"},
				},
			},
			{
				Id:        1004,
				BeginsAt:  fixedNow.Add(-1 * time.Hour),
				Duration:  15 * time.Minute,
				Type:      "outage",
				State:     "resolved",
				Impact:    "full",
				EnSummary: "Recent outage",
				AffectedEntities: []OutageEntity{
					{Name: "location", Id: 4, Label: "Brno"},
				},
			},
		},
	}

	rr := getThroughRoutes(t, app, "/")
	requireStatus(t, rr, http.StatusOK)

	body := rr.Body.String()
	requireContains(t, body, "Reported", "Resolved", "Switch down", "Old maintenance")
	requireOccurrences(t, body, "Maintenances and Outages", 2)
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
	requireJSONString(t, raw, "generated_at", "2026-05-02T10:30:00Z")

	rawNotice := requireMapValue(t, raw, "notice")
	requireJSONBool(t, rawNotice, "any", true)
	requireJSONString(t, rawNotice, "text", "<p>Maintenance notice</p>")
	requireJSONString(t, rawNotice, "updated_at", fixedNoticeTime.Local().Format(time.RFC3339))

	rawVpsAdmin := requireMapValue(t, raw, "vpsadmin")
	rawVpsAdminAPI := requireMapValue(t, rawVpsAdmin, "api")
	requireJSONString(t, rawVpsAdminAPI, "label", "https://api.vpsfree.cz")
	requireJSONString(t, rawVpsAdminAPI, "url", "https://api.vpsfree.cz")
	requireJSONString(t, rawVpsAdminAPI, "status", "operational")

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
	rawOutages := requireMapValue(t, raw, "outage_reports")
	rawAnnounced := requireSliceMap(t, requireSliceValue(t, rawOutages, "announced"), 0)
	requireJSONNumber(t, rawAnnounced, "id", 1001)
	requireJSONString(t, rawAnnounced, "begins_at", "2026-05-02T12:30:00Z")
	requireJSONNumber(t, rawAnnounced, "duration", 90)
	requireJSONString(t, rawAnnounced, "type", "maintenance")
	requireJSONString(t, rawAnnounced, "state", "announced")
	requireJSONString(t, rawAnnounced, "impact", "partial")
	requireJSONString(t, rawAnnounced, "en_summary", "Router replacement")
	requireJSONString(t, rawAnnounced, "en_description", "Planned router replacement.")
	rawAnnouncedEntity := requireSliceMap(t, requireSliceValue(t, rawAnnounced, "entities"), 0)
	requireJSONString(t, rawAnnouncedEntity, "name", "node")
	requireJSONNumber(t, rawAnnouncedEntity, "id", 101)
	requireJSONString(t, rawAnnouncedEntity, "label", "node1.prg")
	rawRecent := requireSliceMap(t, requireSliceValue(t, rawOutages, "recent"), 0)
	requireJSONNumber(t, rawRecent, "id", 1002)
	requireJSONString(t, rawRecent, "begins_at", "2026-05-02T09:30:00Z")
	requireJSONNumber(t, rawRecent, "duration", 30)
	requireJSONString(t, rawRecent, "type", "outage")
	requireJSONString(t, rawRecent, "state", "resolved")
	requireJSONString(t, rawRecent, "impact", "full")

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

	rawLocation := requireSliceMap(t, requireSliceValue(t, raw, "locations"), 0)
	requireJSONNumber(t, rawLocation, "id", 3)
	requireJSONString(t, rawLocation, "label", "Praha")
	rawNode := requireSliceMap(t, requireSliceValue(t, rawLocation, "nodes"), 1)
	requireJSONNumber(t, rawNode, "id", 102)
	requireJSONString(t, rawNode, "name", "node2.prg")
	requireJSONNumber(t, rawNode, "location_id", 3)
	requireJSONString(t, rawNode, "os_type", "vpsadminos")
	requireJSONBool(t, rawNode, "vpsadmin", true)
	requireJSONString(t, rawNode, "ping", "degraded")
	requireJSONBool(t, rawNode, "maintenance", false)
	requireJSONString(t, rawNode, "pool_state", "degraded")
	requireJSONString(t, rawNode, "pool_scan", "resilver")
	requireJSONNumber(t, rawNode, "pool_scan_percent", 42.5)
	requireJSONBool(t, rawNode, "pool_status", true)
	rawWebService := requireSliceMap(t, requireSliceValue(t, raw, "web_services"), 1)
	requireJSONString(t, rawWebService, "label", "kb.vpsfree.cz")
	requireJSONString(t, rawWebService, "description", "Knowledge Base in Czech")
	requireJSONString(t, rawWebService, "url", "https://kb.vpsfree.cz")
	requireJSONString(t, rawWebService, "status", "down")
	rawNameServer := requireSliceMap(t, requireSliceValue(t, raw, "nameservers"), 0)
	requireJSONString(t, rawNameServer, "name", "ns1.vpsfree.cz")
	requireJSONString(t, rawNameServer, "ping", "down")
	requireJSONBool(t, rawNameServer, "lookup", true)
}

func TestRoutesServeJSONEmptyNoticeAndOutageShape(t *testing.T) {
	app, st, _ := newTestApplication(t)
	setOperationalFixture(st)

	rr := getThroughRoutes(t, app, "/json")
	requireStatus(t, rr, http.StatusOK)

	var raw map[string]any
	if err := stdjson.Unmarshal(rr.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode raw JSON: %v", err)
	}

	notice := requireMapValue(t, raw, "notice")
	requireJSONBool(t, notice, "any", false)
	requireJSONString(t, notice, "text", "")
	requireJSONString(t, notice, "updated_at", "0001-01-01T00:00:00Z")

	outages := requireMapValue(t, raw, "outage_reports")
	requireJSONBool(t, outages, "status", true)
	requireSliceLength(t, requireSliceValue(t, outages, "announced"), 0)
	requireSliceLength(t, requireSliceValue(t, outages, "recent"), 0)
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
