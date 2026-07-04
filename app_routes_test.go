package main

import (
	"bytes"
	"compress/gzip"
	stdjson "encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
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

func TestRoutesRedirectHTMLToCanonicalLanguage(t *testing.T) {
	app, st, _ := newTestApplication(t)
	setOperationalFixture(st)

	rr := getThroughRoutesWithHeadersRaw(t, app, "/", map[string]string{
		"Accept-Language": "cs-CZ,cs;q=0.9,en;q=0.8",
	})
	requireStatus(t, rr, http.StatusFound)
	if got := rr.Header().Get("Location"); got != "/?lang=en" {
		t.Fatalf("Location = %q, want /?lang=en", got)
	}
	if got := rr.Header().Values("Vary"); !containsString(got, "Accept-Language") {
		t.Fatalf("Vary = %#v, want Accept-Language", got)
	}

	rr = getThroughRoutesRaw(t, app, "/entity?kind=node&id=node1.prg&lang=de")
	requireStatus(t, rr, http.StatusFound)
	if got := rr.Header().Get("Location"); got != "/entity?id=node1.prg&kind=node&lang=en" {
		t.Fatalf("Location = %q, want canonical English entity URL", got)
	}
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
		`.group-detail-link:hover, .group-detail-link:focus-visible { text-decoration: underline !important; }`,
		`<a class="text-body text-decoration-none group-detail-link" href="/group?kind=vpsadmin&amp;lang=en">vpsAdmin`,
		`<a class="text-body text-decoration-none group-detail-link" href="/group?id=3&amp;kind=location&amp;lang=en">Praha`,
		`<a class="text-body text-decoration-none group-detail-link" href="/group?id=4&amp;kind=location&amp;lang=en">Brno`,
		`<a class="text-body text-decoration-none group-detail-link" href="/group?kind=services&amp;lang=en">Services`,
		"node1.prg",
		`href="/entity?id=node1.prg&amp;kind=node&amp;lang=en"`,
		"node2.prg",
		"resolver-prg",
		"vpsfree.cz",
		"ns1.vpsfree.cz",
		"history-day-ok",
		`max-width: min(22rem, calc(100vw - 1rem))`,
		`max-height: calc(100vh - 1rem)`,
		`function positionPopover(wrap)`,
		`aria-label="vpsAdmin history"`,
		`aria-label="Praha history"`,
		`aria-label="Brno history"`,
		`aria-label="Services history"`,
		"history-bar-split",
		"history-lane",
		`aria-label="Operational"`,
		`aria-label="Responding"`,
	)
	requireStatusCountsAfter(t, body, `href="/group?kind=vpsadmin&amp;lang=en"`, StatusCounts{Operational: 3, Total: 3})
	requireStatusCountsAfter(t, body, `data-bs-target="#collapse-vpsadmin"`, StatusCounts{Operational: 3, Total: 3})
	requireStatusCountsAfter(t, body, `href="/group?id=3&amp;kind=location&amp;lang=en"`, StatusCounts{Operational: 3, Total: 3})
	requireStatusCountsAfter(t, body, `data-bs-target="#collapse-nodes-3"`, StatusCounts{Operational: 2, Total: 2})
	requireStatusCountsAfter(t, body, `data-bs-target="#collapse-dns-3"`, StatusCounts{Operational: 1, Total: 1})
	requireStatusCountsAfter(t, body, `href="/group?id=4&amp;kind=location&amp;lang=en"`, StatusCounts{Operational: 1, Total: 1})
	requireStatusCountsAfter(t, body, `href="/group?kind=services&amp;lang=en"`, StatusCounts{Operational: 3, Total: 3})
	requireStatusCountsAfter(t, body, `data-bs-target="#collapse-webservices"`, StatusCounts{Operational: 2, Total: 2})
	requireStatusCountsAfter(t, body, `data-bs-target="#collapse-nameservers"`, StatusCounts{Operational: 1, Total: 1})
	requireNotContains(t, body, "Reported Planned Outages", "Recent Security Advisories", "Unable to fetch outage reports", "Unable to fetch security advisories", "Last 90 days", "Overall status history")
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
		`<a class="navbar-link" href="/?lang=en">Back to Status</a>`,
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
	requireNotContains(t, body, `aria-label="Probe log pages"`)
	if strings.Index(body, "Availability") > strings.Index(body, "Probe log") {
		t.Fatalf("availability should be rendered above probe log")
	}
}

func TestRoutesServeEntityDetailProbeLogCoverage(t *testing.T) {
	app, st, _ := newTestApplication(t)
	setOperationalFixture(st)

	if err := st.History.ReplaceOutages([]*OutageReport{
		testHistoryOutage(7301, fixedNow.Add(-20*time.Minute), "Node outage", []OutageEntity{
			{Name: "Node", Id: 101, Label: "Node node1.prg"},
		}),
	}, fixedNow); err != nil {
		t.Fatalf("replace outages: %v", err)
	}

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
		"Covered by",
		"Unplanned outage: Node outage",
		`href="https://vpsadmin.vpsfree.cz/?page=outage&amp;action=show&amp;id=7301"`,
		"text-bg-danger",
		"probe-covered-danger",
	)
}

func TestRoutesPaginateEntityProbeLog(t *testing.T) {
	app, st, _ := newTestApplication(t)
	setOperationalFixture(st)

	recordProbeLogEvents(t, st, ProbeTarget{
		EntityKind:  historyEntityNode,
		EntityID:    "node1.prg",
		EntityLabel: "node1.prg",
		Method:      "Ping",
	}, 55, fixedNow.Add(-2*time.Hour))

	rr := getThroughRoutes(t, app, "/entity?kind=node&id=node1.prg")
	requireStatus(t, rr, http.StatusOK)
	body := rr.Body.String()
	requireContains(t, body, "event 54", "event 05", "probe_page=2", "Next")
	requireNotContains(t, body, "event 04")

	rr = getThroughRoutes(t, app, "/entity?kind=node&id=node1.prg&probe_page=2")
	requireStatus(t, rr, http.StatusOK)
	body = rr.Body.String()
	requireContains(t, body, "event 04", "event 00", `aria-current="page">2`)
	requireNotContains(t, body, "event 54")
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

func TestRoutesServeGroupDetails(t *testing.T) {
	app, st, _ := newTestApplication(t)
	setOperationalFixture(st)

	windowStart := fixedNow.AddDate(0, 0, -30)
	if err := st.History.ReplaceOutages([]*OutageReport{
		availabilityTestOutage(7101, windowStart.Add(24*time.Hour), 48*time.Hour, []OutageEntity{
			{Name: "Node", Id: 101, Label: "Node node1.prg"},
		}),
	}, fixedNow); err != nil {
		t.Fatalf("replace outages: %v", err)
	}

	records := []struct {
		target ProbeTarget
		status string
		at     time.Time
	}{
		{
			target: ProbeTarget{EntityKind: historyEntityNode, EntityID: "node1.prg", EntityLabel: "node1.prg", Method: "Ping"},
			status: historyProbeStateOperational,
			at:     windowStart.Add(-time.Hour),
		},
		{
			target: ProbeTarget{EntityKind: historyEntityNode, EntityID: "node2.prg", EntityLabel: "node2.prg", Method: "Ping"},
			status: historyProbeStateOperational,
			at:     windowStart.Add(-time.Hour),
		},
		{
			target: ProbeTarget{EntityKind: historyEntityNode, EntityID: "node2.prg", EntityLabel: "node2.prg", Method: "Ping"},
			status: historyProbeStateDown,
			at:     windowStart.Add(24 * time.Hour),
		},
		{
			target: ProbeTarget{EntityKind: historyEntityNode, EntityID: "node2.prg", EntityLabel: "node2.prg", Method: "Ping"},
			status: historyProbeStateOperational,
			at:     windowStart.Add(72 * time.Hour),
		},
		{
			target: ProbeTarget{EntityKind: historyEntityDnsResolver, EntityID: "resolver-prg", EntityLabel: "resolver-prg", Method: "Lookup"},
			status: historyProbeStateOperational,
			at:     windowStart.Add(-time.Hour),
		},
	}
	for _, record := range records {
		if err := st.History.RecordProbeStatus(record.target, record.status, record.status, record.at); err != nil {
			t.Fatalf("record probe status: %v", err)
		}
	}

	rr := getThroughRoutes(t, app, "/group?kind=location&id=3")
	requireStatus(t, rr, http.StatusOK)

	body := rr.Body.String()
	requireContains(
		t,
		body,
		"Praha",
		"Location",
		`aria-label="Praha history"`,
		"Availability",
		"Reported",
		"Probe",
		"96.667%",
		"97.778%",
		"Probe log",
		"Entity",
		"node2.prg",
		"resolver-prg",
	)
}

func TestRoutesServeGroupDetailCombinedProbeLog(t *testing.T) {
	app, st, _ := newTestApplication(t)
	setOperationalFixture(st)

	records := []struct {
		target ProbeTarget
		status string
		at     time.Time
	}{
		{
			target: ProbeTarget{EntityKind: historyEntityNode, EntityID: "node1.prg", EntityLabel: "node1.prg", Method: "Ping"},
			status: historyProbeStateDown,
			at:     fixedNow.Add(-10 * time.Minute),
		},
		{
			target: ProbeTarget{EntityKind: historyEntityDnsResolver, EntityID: "resolver-prg", EntityLabel: "resolver-prg", Method: "Lookup"},
			status: historyProbeStateDown,
			at:     fixedNow.Add(-5 * time.Minute),
		},
	}
	for _, record := range records {
		if err := st.History.RecordProbeStatus(record.target, record.status, record.status, record.at); err != nil {
			t.Fatalf("record probe status: %v", err)
		}
	}

	rr := getThroughRoutes(t, app, "/group?kind=location&id=3")
	requireStatus(t, rr, http.StatusOK)

	body := rr.Body.String()
	resolverIndex := strings.Index(body, "<td>resolver-prg</td>")
	nodeIndex := strings.Index(body, "<td>node1.prg</td>")
	if resolverIndex == -1 || nodeIndex == -1 {
		t.Fatalf("expected resolver and node probe rows in body:\n%s", body)
	}
	if resolverIndex > nodeIndex {
		t.Fatalf("probe log should be sorted newest first")
	}
}

func TestRoutesPaginateGroupProbeLog(t *testing.T) {
	app, st, _ := newTestApplication(t)
	setOperationalFixture(st)

	recordProbeLogEvents(t, st, ProbeTarget{
		EntityKind:  historyEntityNode,
		EntityID:    "node1.prg",
		EntityLabel: "node1.prg",
		Method:      "Ping",
	}, 51, fixedNow.Add(-2*time.Hour))

	rr := getThroughRoutes(t, app, "/group?kind=location&id=3")
	requireStatus(t, rr, http.StatusOK)
	body := rr.Body.String()
	requireContains(t, body, "event 50", "probe_page=2", `href="/group?id=3&amp;kind=location&amp;lang=en&amp;probe_page=2"`)
	requireNotContains(t, body, "event 00")

	rr = getThroughRoutes(t, app, "/group?kind=location&id=3&probe_page=2")
	requireStatus(t, rr, http.StatusOK)
	body = rr.Body.String()
	requireContains(t, body, "event 00", `href="/group?id=3&amp;kind=location&amp;lang=en"`)
	requireNotContains(t, body, "event 50")
}

func TestRoutesServeServiceGroupDetailHidesReportedAvailability(t *testing.T) {
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

	rr := getThroughRoutes(t, app, "/group?kind=services&amp;lang=en")
	requireStatus(t, rr, http.StatusOK)

	body := rr.Body.String()
	requireContains(t, body, "Services", "Availability", "Probe", "100.000%")
	requireNotContains(t, body, "Reported")
}

func TestRoutesServeVpsAdminGroupDetail(t *testing.T) {
	app, st, _ := newTestApplication(t)
	setOperationalFixture(st)

	rr := getThroughRoutes(t, app, "/group?kind=vpsadmin&amp;lang=en")
	requireStatus(t, rr, http.StatusOK)

	requireContains(
		t,
		rr.Body.String(),
		"vpsAdmin",
		"Group",
		"Availability",
		"Reported",
		"Probe",
		"100.000%",
	)
}

func TestRoutesServeHistoryPopoverWithOutageLinks(t *testing.T) {
	app, st, _ := newTestApplication(t)
	setOperationalFixture(st)
	beginsAt := fixedNow.Add(-24 * time.Hour)

	if err := st.History.ReplaceOutages([]*OutageReport{
		{
			Id:        2001,
			BeginsAt:  beginsAt,
			Duration:  30 * time.Minute,
			Type:      "unplanned_outage",
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
		"Unplanned outage: Power failure",
		"Started: "+beginsAt.Local().Format(historyIncidentTimeFormat),
		"Expected duration: 30 min",
		`href="https://vpsadmin.vpsfree.cz/?page=outage&amp;action=show&amp;id=2001"`,
		`target="_blank"`,
	)
}

func TestRoutesServeEntityDetailNotFound(t *testing.T) {
	app, st, _ := newTestApplication(t)
	setOperationalFixture(st)

	rr := getThroughRoutes(t, app, "/entity?kind=node&id=missing")
	requireStatus(t, rr, http.StatusNotFound)
	if got := rr.Header().Get("Cache-Control"); got != "" {
		t.Fatalf("missing entity Cache-Control = %q, want empty", got)
	}

	rr = getThroughRoutes(t, app, "/group?kind=location&id=999")
	requireStatus(t, rr, http.StatusNotFound)
	if got := rr.Header().Get("Cache-Control"); got != "" {
		t.Fatalf("missing group Cache-Control = %q, want empty", got)
	}

	rr = getThroughRoutes(t, app, "/group?kind=unknown")
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
		"kb.vpsfree.cz",
		`aria-label="Down"`,
	)
	body := rr.Body.String()
	requireStatusCountsAfter(t, body, `href="/group?id=3&amp;kind=location&amp;lang=en"`, StatusCounts{Operational: 2, Down: 1, Total: 3})
	requireStatusCountsAfter(t, body, `data-bs-target="#collapse-nodes-3"`, StatusCounts{Operational: 1, Down: 1, Total: 2})
	requireStatusCountsAfter(t, body, `href="/group?kind=services&amp;lang=en"`, StatusCounts{Operational: 2, Down: 1, Total: 3})
	requireStatusCountsAfter(t, body, `data-bs-target="#collapse-webservices"`, StatusCounts{Operational: 1, Down: 1, Total: 2})
}

func TestRoutesServeIndexTreatsMissingVpsAdminNodeStatusAsDegradedWhenPingResponds(t *testing.T) {
	app, st, _ := newTestApplication(t)
	setOperationalFixture(st)

	setVpsAdminService(st, "api", st.VpsAdmin.Api, false, false, http.StatusInternalServerError)
	node := st.GlobalNodeMap["node2.prg"]
	setNodeState(st, st.LocationMap["Praha"], node, false, false, "vpsadminos", false, "unknown", "none", 0, 0)

	rr := getThroughRoutes(t, app, "/")
	requireStatus(t, rr, http.StatusOK)

	body := rr.Body.String()
	requireContains(t, body, "Unable to determine storage status")
	requireStatusCountsAfter(t, body, `href="/group?kind=vpsadmin&amp;lang=en"`, StatusCounts{Operational: 2, Down: 1, Total: 3})
	requireStatusCountsAfter(t, body, `href="/group?id=3&amp;kind=location&amp;lang=en"`, StatusCounts{Operational: 2, Degraded: 1, Total: 3})
	requireStatusCountsAfter(t, body, `data-bs-target="#collapse-nodes-3"`, StatusCounts{Operational: 1, Degraded: 1, Total: 2})
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
		"Planned Outages",
		"Resolved",
		"Unplanned Outages",
		"Router replacement",
		"Power failure",
		"node1.prg",
		"https://vpsadmin.vpsfree.cz/?page=outage&action=show&id=1001",
		`aria-label="Under maintenance"`,
		`aria-label="Degraded"`,
		`Storage is being resilvered to replace disks, 42.5 % done`,
	)
	body := rr.Body.String()
	requireStatusCountsAfter(t, body, `href="/group?kind=vpsadmin&amp;lang=en"`, StatusCounts{Operational: 2, Degraded: 1, Total: 3})
	requireStatusCountsAfter(t, body, `href="/group?id=3&amp;kind=location&amp;lang=en"`, StatusCounts{Operational: 2, Degraded: 1, Total: 3})
	requireStatusCountsAfter(t, body, `data-bs-target="#collapse-nodes-3"`, StatusCounts{Operational: 1, Degraded: 1, Total: 2})
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

func TestRoutesServeIndexSecurityAdvisories(t *testing.T) {
	app, st, _ := newTestApplication(t)
	setOperationalFixture(st)
	addSecurityAdvisoryFixture(st)

	rr := getThroughRoutes(t, app, "/")
	requireStatus(t, rr, http.StatusOK)

	body := rr.Body.String()
	requireContains(
		t,
		body,
		"Recent Security Advisories",
		"2026-05-02",
		"CVE-2026-2001 (Dirty Pipe)",
		"Kernel vulnerability was mitigated on all affected nodes.",
		"https://vpsadmin.vpsfree.cz/?page=security_advisory&action=show&id=2001",
		"No issues reported.",
	)
	requireNotContains(t, body, "Affected nodes")
	requireBefore(t, body, "No issues reported.", "Recent Security Advisories")
}

func TestRoutesServeIndexSecurityAdvisoryFetchFailureDoesNotSuppressNoIssues(t *testing.T) {
	app, st, _ := newTestApplication(t)
	setOperationalFixture(st)
	st.SecurityAdvisories.Status = false

	rr := getThroughRoutes(t, app, "/")
	requireStatus(t, rr, http.StatusOK)

	body := rr.Body.String()
	requireContains(
		t,
		body,
		"No issues reported.",
		"Unable to fetch security advisories from vpsAdmin.",
	)
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
		`aria-label="Under maintenance"`,
		`aria-label="Not supported"`,
		"Storage not operational",
		"Storage is being scrubbed to check data integrity, 15.0 % done",
	)
	body := rr.Body.String()
	requireStatusCountsAfter(t, body, `href="/group?id=3&amp;kind=location&amp;lang=en"`, StatusCounts{Operational: 2, Down: 1, Total: 3})
	requireStatusCountsAfter(t, body, `data-bs-target="#collapse-nodes-3"`, StatusCounts{Operational: 1, Down: 1, Total: 2})
	requireStatusCountsAfter(t, body, `href="/group?kind=services&amp;lang=en"`, StatusCounts{Operational: 2, Degraded: 1, Total: 3})
	requireStatusCountsAfter(t, body, `data-bs-target="#collapse-webservices"`, StatusCounts{Operational: 1, Degraded: 1, Total: 2})
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
		"Storage is being scrubbed to check data integrity, 15.0 % done",
	)
	requireStatusCountsAfter(t, body, `href="/group?id=3&amp;kind=location&amp;lang=en"`, StatusCounts{Operational: 3, Total: 3})
	requireStatusCountsAfter(t, body, `data-bs-target="#collapse-nodes-3"`, StatusCounts{Operational: 2, Total: 2})

	prahaIndex := strings.Index(body, `href="/group?id=3&amp;kind=location&amp;lang=en"`)
	if prahaIndex == -1 {
		t.Fatalf("Praha heading not found in body")
	}
	prahaHeadingStart := strings.LastIndex(body[:prahaIndex], "<h2")
	if prahaHeadingStart == -1 {
		t.Fatalf("Praha heading tag start not found in body")
	}
	prahaHeadingTagEnd := strings.Index(body[prahaHeadingStart:], ">")
	if prahaHeadingTagEnd == -1 {
		t.Fatalf("Praha heading tag boundaries not found in body")
	}
	prahaHeadingTag := body[prahaHeadingStart : prahaHeadingStart+prahaHeadingTagEnd]
	if strings.Contains(prahaHeadingTag, `text-success`) || strings.Contains(prahaHeadingTag, `text-warning`) || strings.Contains(prahaHeadingTag, `text-danger`) {
		t.Fatalf("Praha heading tag = %q, should not have state color", prahaHeadingTag)
	}

	nodesIndex := strings.Index(body, `data-bs-target="#collapse-nodes-3"`)
	if nodesIndex == -1 {
		t.Fatalf("Nodes heading not found in body")
	}
	nodesButtonStart := strings.LastIndex(body[:nodesIndex], "<button")
	if nodesButtonStart == -1 {
		t.Fatalf("Nodes button tag start not found in body")
	}
	nodesButtonTagEnd := strings.Index(body[nodesButtonStart:], ">")
	if nodesButtonTagEnd == -1 {
		t.Fatalf("Nodes button tag boundaries not found in body")
	}
	nodesButtonTag := body[nodesButtonStart : nodesButtonStart+nodesButtonTagEnd]
	if strings.Contains(nodesButtonTag, `text-success`) || strings.Contains(nodesButtonTag, `text-warning`) || strings.Contains(nodesButtonTag, `text-danger`) {
		t.Fatalf("Nodes button tag = %q, should not have state color", nodesButtonTag)
	}

	nodeIndex := strings.Index(body, `href="/entity?id=node2.prg&amp;kind=node&amp;lang=en">node2.prg</a>`)
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
		Status:             true,
		AnyActive:          true,
		AnyActivePlanned:   true,
		AnyActiveUnplanned: true,
		AnyRecent:          true,
		AnyRecentPlanned:   true,
		AnyRecentUnplanned: true,
		ActiveList: []*OutageReport{
			{
				Id:        1001,
				BeginsAt:  fixedNow.Add(2 * time.Hour),
				Duration:  90 * time.Minute,
				Type:      "planned_outage",
				State:     "announced",
				Impact:    "system_restart",
				EnSummary: "Router replacement",
				AffectedEntities: []OutageEntity{
					{Name: "node", Id: 101, Label: "node1.prg"},
				},
			},
			{
				Id:        1002,
				BeginsAt:  fixedNow.Add(3 * time.Hour),
				Duration:  45 * time.Minute,
				Type:      "unplanned_outage",
				State:     "announced",
				Impact:    "unavailability",
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
				Type:      "planned_outage",
				State:     "resolved",
				Impact:    "system_restart",
				EnSummary: "Old maintenance",
				AffectedEntities: []OutageEntity{
					{Name: "node", Id: 102, Label: "node2.prg"},
				},
			},
			{
				Id:        1004,
				BeginsAt:  fixedNow.Add(-1 * time.Hour),
				Duration:  15 * time.Minute,
				Type:      "unplanned_outage",
				State:     "resolved",
				Impact:    "unavailability",
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
	requireContains(t, body, "Reported", "Resolved", "Switch down", "Old maintenance", "System restart", "Unavailability")
	requireNotContains(t, body, "system_restart", "unavailability")
	requireOccurrences(t, body, "Planned and Unplanned Outages", 2)
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
	st.SecurityAdvisories.Status = false

	rr := getThroughRoutes(t, app, "/")
	requireStatus(t, rr, http.StatusOK)

	requireContains(
		t,
		rr.Body.String(),
		"Unable to fetch outage reports from vpsAdmin.",
		"Unable to fetch security advisories from vpsAdmin.",
		`aria-label="Down"`,
		`aria-label="Error"`,
	)
	body := rr.Body.String()
	requireStatusCountsAfter(t, body, `href="/group?kind=vpsadmin&amp;lang=en"`, StatusCounts{Down: 3, Total: 3})
	requireStatusCountsAfter(t, body, `href="/group?id=3&amp;kind=location&amp;lang=en"`, StatusCounts{Down: 3, Total: 3})
	requireStatusCountsAfter(t, body, `href="/group?id=4&amp;kind=location&amp;lang=en"`, StatusCounts{Down: 1, Total: 1})
	requireStatusCountsAfter(t, body, `href="/group?kind=services&amp;lang=en"`, StatusCounts{Down: 3, Total: 3})
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
		`href="/about?lang=en"`,
		"https://github.com/vpsfreecz/vpsf-status",
	)

	static := getThroughRoutes(t, app, "/static/favicon.png")
	requireStatus(t, static, http.StatusOK)
	if got := static.Header().Get("Cache-Control"); got != "public, max-age=86400" {
		t.Fatalf("static Cache-Control = %q, want public, max-age=86400", got)
	}
	if static.Body.Len() == 0 {
		t.Fatal("static favicon response body is empty")
	}
}

func TestRoutesServePreRenderedIndexUntilRefreshed(t *testing.T) {
	now := fixedNow
	app, st, _ := newTestApplication(t)
	app.now = func() time.Time {
		return now
	}
	setOperationalFixture(st)

	if _, err := app.refreshIndexResponse(now); err != nil {
		t.Fatalf("pre-render index: %v", err)
	}
	requireUnlabeledMetricValue(t, scrapeMetrics(t, app)["vpsfstatus_index_last_render_timestamp_seconds"], float64(now.Unix()))

	first := getThroughRoutes(t, app, "/")
	requireStatus(t, first, http.StatusOK)
	requireContains(t, first.Body.String(), "Rendered at: Sat May  2 10:30:00 UTC 2026")

	setWebServiceState(st, st.Services.Web[1], false, false, http.StatusInternalServerError)
	now = fixedNow.Add(2 * time.Second)

	stale := getThroughRoutes(t, app, "/")
	requireStatus(t, stale, http.StatusOK)
	requireContains(t, stale.Body.String(), "No issues reported.", "Rendered at: Sat May  2 10:30:02 UTC 2026")
	requireNotContains(t, stale.Body.String(), `aria-label="Down"`)

	if _, err := app.refreshIndexResponse(now); err != nil {
		t.Fatalf("refresh index: %v", err)
	}
	requireUnlabeledMetricValue(t, scrapeMetrics(t, app)["vpsfstatus_index_last_render_timestamp_seconds"], float64(now.Unix()))

	fresh := getThroughRoutes(t, app, "/")
	requireStatus(t, fresh, http.StatusOK)
	requireContains(t, fresh.Body.String(), `aria-label="Down"`, "Rendered at: Sat May  2 10:30:02 UTC 2026")
}

func TestRoutesCacheDynamicJSONResponsesForOneSecond(t *testing.T) {
	now := fixedNow
	app, st, _ := newTestApplication(t)
	app.now = func() time.Time {
		return now
	}
	setOperationalFixture(st)

	first := getThroughRoutes(t, app, "/json")
	requireStatus(t, first, http.StatusOK)
	requireGeneratedAt(t, first, fixedNow)

	now = fixedNow.Add(500 * time.Millisecond)
	second := getThroughRoutes(t, app, "/json")
	requireStatus(t, second, http.StatusOK)
	requireGeneratedAt(t, second, fixedNow)

	now = fixedNow.Add(2 * time.Second)
	third := getThroughRoutes(t, app, "/json")
	requireStatus(t, third, http.StatusOK)
	requireGeneratedAt(t, third, fixedNow.Add(2*time.Second))
}

func TestRoutesRefreshNoticeCacheAfterTTL(t *testing.T) {
	now := fixedNow
	app, st, cfg := newTestApplication(t)
	app.now = func() time.Time {
		return now
	}
	setOperationalFixture(st)

	writeNotice(t, cfg, "<p>First notice</p>")
	first := getThroughRoutes(t, app, "/json")
	requireStatus(t, first, http.StatusOK)
	requireContains(t, first.Body.String(), "First notice")

	writeNotice(t, cfg, "<p>Second notice with different size</p>")
	now = now.Add(500 * time.Millisecond)
	second := getThroughRoutes(t, app, "/json")
	requireStatus(t, second, http.StatusOK)
	requireContains(t, second.Body.String(), "First notice")

	now = now.Add(2 * time.Second)
	third := getThroughRoutes(t, app, "/json")
	requireStatus(t, third, http.StatusOK)
	requireContains(t, third.Body.String(), "Second notice with different size")
}

func TestRoutesServeGzipForDynamicResponses(t *testing.T) {
	app, st, _ := newTestApplication(t)
	setOperationalFixture(st)
	addOutageFixture(st)
	addSecurityAdvisoryFixture(st)

	rr := getThroughRoutesWithHeaders(t, app, "/json", map[string]string{
		"Accept-Encoding": "gzip",
	})
	requireStatus(t, rr, http.StatusOK)

	if got := rr.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
	if got := rr.Header().Values("Vary"); !headerValuesContain(got, "Accept-Encoding") {
		t.Fatalf("Vary = %v, want Accept-Encoding", got)
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}

	body := gunzipBody(t, rr.Body.Bytes())
	var raw map[string]any
	if err := stdjson.Unmarshal(body, &raw); err != nil {
		t.Fatalf("decode gzipped JSON: %v\n%s", err, string(body))
	}
	requireJSONContractKeys(t, raw)
}

func TestRoutesServeGzipForPreRenderedIndex(t *testing.T) {
	app, st, _ := newTestApplication(t)
	setOperationalFixture(st)

	if _, err := app.refreshIndexResponse(app.currentTime()); err != nil {
		t.Fatalf("pre-render index: %v", err)
	}

	rr := getThroughRoutesWithHeaders(t, app, "/", map[string]string{
		"Accept-Encoding": "gzip",
	})
	requireStatus(t, rr, http.StatusOK)

	if got := rr.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
	if got := rr.Header().Values("Vary"); !headerValuesContain(got, "Accept-Encoding") {
		t.Fatalf("Vary = %v, want Accept-Encoding", got)
	}
	if got := rr.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want text/html; charset=utf-8", got)
	}

	body := string(gunzipBody(t, rr.Body.Bytes()))
	requireContains(t, body, "No issues reported.", "Rendered at: Sat May  2 10:30:00 UTC 2026")
}

func TestRoutesDoNotCacheMetrics(t *testing.T) {
	app, st, _ := newTestApplication(t)
	setOperationalFixture(st)

	rr := getThroughRoutes(t, app, "/metrics")
	requireStatus(t, rr, http.StatusOK)
	if got := rr.Header().Get("Cache-Control"); got != "" {
		t.Fatalf("metrics Cache-Control = %q, want empty", got)
	}
	if got := rr.Header().Values("Vary"); headerValuesContain(got, "Accept-Encoding") {
		t.Fatalf("metrics Vary = %v, should not be set by route cache", got)
	}
}

func TestRoutesServeJSONContract(t *testing.T) {
	app, st, cfg := newTestApplication(t)
	setOperationalFixture(st)
	writeNotice(t, cfg, "<p>Maintenance notice</p>")
	addOutageFixture(st)
	addSecurityAdvisoryFixture(st)

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
	if got := body.OutageReports.Announced[0]; got.Id != 1001 || got.Type != "planned_outage" || got.State != "announced" || got.Entities[0].Label != "node1.prg" {
		t.Fatalf("announced outage = %+v", got)
	}
	if got := body.OutageReports.Recent[0]; got.Id != 1002 || got.Type != "unplanned_outage" || got.State != "resolved" || got.Entities[0].Label != "Praha" {
		t.Fatalf("recent outage = %+v", got)
	}
	rawOutages := requireMapValue(t, raw, "outage_reports")
	rawAnnounced := requireSliceMap(t, requireSliceValue(t, rawOutages, "announced"), 0)
	requireJSONNumber(t, rawAnnounced, "id", 1001)
	requireJSONString(t, rawAnnounced, "begins_at", "2026-05-02T12:30:00Z")
	requireJSONNumber(t, rawAnnounced, "duration", 90)
	requireJSONString(t, rawAnnounced, "type", "planned_outage")
	requireJSONString(t, rawAnnounced, "state", "announced")
	requireJSONString(t, rawAnnounced, "impact", "system_restart")
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
	requireJSONString(t, rawRecent, "type", "unplanned_outage")
	requireJSONString(t, rawRecent, "state", "resolved")
	requireJSONString(t, rawRecent, "impact", "unavailability")

	if !body.SecurityAdvisories.Status || len(body.SecurityAdvisories.Recent) != 1 {
		t.Fatalf("security_advisories = %+v", body.SecurityAdvisories)
	}
	if got := body.SecurityAdvisories.Recent[0]; got.Id != 2001 || len(got.Cves) != 1 || got.Cves[0].CveId != "CVE-2026-2001" || got.Name != "Dirty Pipe" || got.AffectedNodeCount != 2 {
		t.Fatalf("recent security advisory = %+v", got)
	}
	rawSecurityAdvisories := requireMapValue(t, raw, "security_advisories")
	rawSecurityAdvisory := requireSliceMap(t, requireSliceValue(t, rawSecurityAdvisories, "recent"), 0)
	requireJSONNumber(t, rawSecurityAdvisory, "id", 2001)
	requireJSONString(t, rawSecurityAdvisory, "published_at", "2026-05-02T08:30:00Z")
	requireJSONString(t, rawSecurityAdvisory, "updated_at", "2026-05-02T09:00:00Z")
	requireJSONString(t, rawSecurityAdvisory, "state", "published")
	rawCve := requireSliceMap(t, requireSliceValue(t, rawSecurityAdvisory, "cves"), 0)
	requireJSONNumber(t, rawCve, "id", 3001)
	requireJSONString(t, rawCve, "cve_id", "CVE-2026-2001")
	requireJSONString(t, rawCve, "url", "https://www.cve.org/CVERecord?id=CVE-2026-2001")
	requireJSONString(t, rawSecurityAdvisory, "name", "Dirty Pipe")
	requireJSONString(t, rawSecurityAdvisory, "en_summary", "Kernel vulnerability was mitigated on all affected nodes.")
	requireJSONNumber(t, rawSecurityAdvisory, "affected_node_count", 2)

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

	advisories := requireMapValue(t, raw, "security_advisories")
	requireJSONBool(t, advisories, "status", true)
	requireSliceLength(t, requireSliceValue(t, advisories, "recent"), 0)
}

func getThroughRoutesWithHeaders(t *testing.T, app *application, target string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	return getThroughRoutesWithHeadersRaw(t, app, canonicalTestTarget(t, target), headers)
}

func getThroughRoutesWithHeadersRaw(t *testing.T, app *application, target string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	rr := httptest.NewRecorder()
	app.routes().ServeHTTP(rr, req)
	return rr
}

func recordProbeLogEvents(t *testing.T, st *Status, target ProbeTarget, count int, start time.Time) {
	t.Helper()

	for i := 0; i < count; i++ {
		status := historyProbeStateOperational
		if i%2 == 1 {
			status = historyProbeStateDown
		}
		if err := st.History.RecordProbeStatus(target, status, fmt.Sprintf("event %02d", i), start.Add(time.Duration(i)*time.Minute)); err != nil {
			t.Fatalf("record probe event %d: %v", i, err)
		}
	}
}

func gunzipBody(t *testing.T, body []byte) []byte {
	t.Helper()

	zr, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("open gzip body: %v", err)
	}
	defer zr.Close()

	ret, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("read gzip body: %v", err)
	}
	return ret
}

func requireGeneratedAt(t *testing.T, rr *httptest.ResponseRecorder, want time.Time) {
	t.Helper()

	var body statusjson.Status
	if err := stdjson.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode JSON: %v\n%s", err, rr.Body.String())
	}
	if !body.GeneratedAt.Equal(want) {
		t.Fatalf("generated_at = %s, want %s", body.GeneratedAt, want)
	}
}

func headerValuesContain(values []string, want string) bool {
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			if strings.EqualFold(strings.TrimSpace(part), want) {
				return true
			}
		}
	}
	return false
}

func requireJSONContractKeys(t *testing.T, raw map[string]any) {
	t.Helper()

	requireMapKeys(t, raw, "generated_at", "locations", "nameservers", "notice", "outage_reports", "security_advisories", "vpsadmin", "web_services")

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

	advisories := requireMapValue(t, raw, "security_advisories")
	requireMapKeys(t, advisories, "recent", "status")
	requireSecurityAdvisoryJSONKeys(t, requireSliceMap(t, requireSliceValue(t, advisories, "recent"), 0))

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

func requireSecurityAdvisoryJSONKeys(t *testing.T, advisory map[string]any) {
	t.Helper()

	requireMapKeys(t, advisory, "affected_node_count", "cves", "en_description", "en_response", "en_summary", "id", "name", "published_at", "state", "updated_at")
	requireMapKeys(t, requireSliceMap(t, requireSliceValue(t, advisory, "cves"), 0), "cve_id", "id", "url")
}
