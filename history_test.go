package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestOpenHistoryStoreRejectsUnusablePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history")
	if err := os.WriteFile(path, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	if _, err := openHistoryStore(path); err == nil {
		t.Fatal("openHistoryStore returned nil error for file path")
	}
}

func TestOpenHistoryStoreCreatesVersionedSQLiteDatabase(t *testing.T) {
	dir := t.TempDir()
	hs, err := openHistoryStore(dir)
	if err != nil {
		t.Fatalf("open history store: %v", err)
	}
	t.Cleanup(func() { hs.Close() })

	if _, err := os.Stat(filepath.Join(dir, historyDBFilename)); err != nil {
		t.Fatalf("stat history database: %v", err)
	}

	version, err := readHistorySchemaVersion(hs.db)
	if err != nil {
		t.Fatalf("read schema version: %v", err)
	}
	if version != historySchemaVersion {
		t.Fatalf("schema version = %d, want %d", version, historySchemaVersion)
	}
}

func TestOpenHistoryStoreEnsuresCurrentObjectsForExistingV1Database(t *testing.T) {
	dir := t.TempDir()
	hs, err := openHistoryStore(dir)
	if err != nil {
		t.Fatalf("open history store: %v", err)
	}
	if _, err := hs.db.Exec(`DROP TABLE entity_snapshots`); err != nil {
		t.Fatalf("drop entity snapshots: %v", err)
	}
	if err := hs.Close(); err != nil {
		t.Fatalf("close history store: %v", err)
	}

	reopened, err := openHistoryStore(dir)
	if err != nil {
		t.Fatalf("reopen history store: %v", err)
	}
	t.Cleanup(func() { reopened.Close() })

	if err := reopened.RecordEntitySnapshots([]HistoryEntitySnapshot{
		{
			EntityKind:         historyEntityNode,
			EntityID:           "node9.stg",
			EntityLabel:        "node9.stg",
			NodeID:             450,
			GroupKind:          historyGroupLocation,
			GroupID:            3,
			GroupLabel:         "Praha",
			VpsAdminLocationID: 7,
			LastSeen:           fixedNow,
		},
	}); err != nil {
		t.Fatalf("record entity snapshot after reopening old schema: %v", err)
	}

	if snapshots := reopened.EntitySnapshots(); len(snapshots) != 1 {
		t.Fatalf("entity snapshots after reopening old schema = %+v, want one", snapshots)
	}
}

func TestOpenHistoryStoreRejectsNewerSchemaVersion(t *testing.T) {
	dir := t.TempDir()
	hs, err := openHistoryStore(dir)
	if err != nil {
		t.Fatalf("open history store: %v", err)
	}

	if _, err := hs.db.Exec(fmt.Sprintf("PRAGMA user_version = %d", historySchemaVersion+1)); err != nil {
		t.Fatalf("set schema version: %v", err)
	}
	if err := hs.Close(); err != nil {
		t.Fatalf("close history store: %v", err)
	}

	_, err = openHistoryStore(dir)
	if err == nil {
		t.Fatal("openHistoryStore returned nil error for newer schema")
	}
	if !strings.Contains(err.Error(), "newer than supported") {
		t.Fatalf("newer schema error = %q", err)
	}
}

func TestOpenHistoryStoreIgnoresOldJSONHistoryFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, historyOutagesFilename), []byte("{"), 0o644); err != nil {
		t.Fatalf("write old outage history: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, historyProbeFilename), []byte("{"), 0o644); err != nil {
		t.Fatalf("write old probe history: %v", err)
	}

	hs, err := openHistoryStore(dir)
	if err != nil {
		t.Fatalf("open history store: %v", err)
	}
	t.Cleanup(func() { hs.Close() })

	if reports := hs.OutageReports(); len(reports) != 0 {
		t.Fatalf("outage reports imported from JSON = %+v, want none", reports)
	}
}

func TestHistoryStoreRecordsProbeChangesAndPromotesIncidents(t *testing.T) {
	hs, err := openHistoryStore(t.TempDir())
	if err != nil {
		t.Fatalf("open history store: %v", err)
	}

	target := ProbeTarget{
		EntityKind:  historyEntityNode,
		EntityID:    "node1.prg",
		EntityLabel: "node1.prg",
		Method:      "Ping",
	}

	if err := hs.RecordProbeStatus(target, historyProbeStateOperational, "responding", fixedNow); err != nil {
		t.Fatalf("record initial status: %v", err)
	}
	if err := hs.RecordProbeStatus(target, historyProbeStateDown, "not responding", fixedNow.Add(time.Minute)); err != nil {
		t.Fatalf("record down status: %v", err)
	}
	if incidents := hs.ProbeIncidents(fixedNow.Add(4*time.Minute), historyDefaultDays); len(incidents) != 0 {
		t.Fatalf("incidents before threshold = %+v, want none", incidents)
	}

	if err := hs.RecordProbeStatus(target, historyProbeStateDown, "not responding", fixedNow.Add(7*time.Minute)); err != nil {
		t.Fatalf("record promoted status: %v", err)
	}
	incidents := hs.ProbeIncidents(fixedNow.Add(7*time.Minute), historyDefaultDays)
	if len(incidents) != 1 || incidents[0].EndsAt != nil || incidents[0].Status != historyProbeStateDown {
		t.Fatalf("promoted incidents = %+v", incidents)
	}

	if err := hs.RecordProbeStatus(target, historyProbeStateOperational, "responding", fixedNow.Add(8*time.Minute)); err != nil {
		t.Fatalf("record recovery: %v", err)
	}
	incidents = hs.ProbeIncidents(fixedNow.Add(8*time.Minute), historyDefaultDays)
	if len(incidents) != 1 || incidents[0].EndsAt == nil || !incidents[0].EndsAt.Equal(fixedNow.Add(8*time.Minute)) {
		t.Fatalf("closed incidents = %+v", incidents)
	}

	events := hs.ProbeEventsFor(historyEntityNode, "node1.prg", fixedNow.Add(8*time.Minute), historyDefaultDays)
	if len(events) != 3 || events[0].Status != historyProbeStateOperational || events[1].Status != historyProbeStateDown || events[2].Status != historyProbeStateOperational {
		t.Fatalf("probe events = %+v", events)
	}
}

func TestHistoryStoreIgnoresTransientProbeFailuresForIncidents(t *testing.T) {
	hs, err := openHistoryStore(t.TempDir())
	if err != nil {
		t.Fatalf("open history store: %v", err)
	}

	target := ProbeTarget{
		EntityKind:  historyEntityWebService,
		EntityID:    "kb.vpsfree.cz",
		EntityLabel: "kb.vpsfree.cz",
		Method:      "HTTP",
	}

	if err := hs.RecordProbeStatus(target, historyProbeStateDown, "HTTP 500", fixedNow); err != nil {
		t.Fatalf("record down status: %v", err)
	}
	if err := hs.RecordProbeStatus(target, historyProbeStateOperational, "HTTP 200", fixedNow.Add(4*time.Minute)); err != nil {
		t.Fatalf("record recovery: %v", err)
	}

	if incidents := hs.ProbeIncidents(fixedNow.Add(4*time.Minute), historyDefaultDays); len(incidents) != 0 {
		t.Fatalf("transient incidents = %+v, want none", incidents)
	}
}

func TestHistoryStoreKeepsProbeHistoryAndQueriesWindow(t *testing.T) {
	hs, err := openHistoryStore(t.TempDir())
	if err != nil {
		t.Fatalf("open history store: %v", err)
	}

	target := ProbeTarget{
		EntityKind:  historyEntityNode,
		EntityID:    "node1.prg",
		EntityLabel: "node1.prg",
		Method:      "Ping",
	}

	oldStart := fixedNow.AddDate(0, 0, -120)
	records := []struct {
		status  string
		message string
		at      time.Time
	}{
		{historyProbeStateOperational, "responding", oldStart},
		{historyProbeStateDown, "not responding", oldStart.Add(time.Minute)},
		{historyProbeStateDown, "not responding", oldStart.Add(7 * time.Minute)},
		{historyProbeStateOperational, "responding", oldStart.Add(8 * time.Minute)},
		{historyProbeStateDown, "not responding", fixedNow.Add(-10 * time.Minute)},
		{historyProbeStateOperational, "responding", fixedNow.Add(-2 * time.Minute)},
	}
	for _, record := range records {
		if err := hs.RecordProbeStatus(target, record.status, record.message, record.at); err != nil {
			t.Fatalf("record probe status: %v", err)
		}
	}

	if got := historyTableCount(t, hs, "probe_events"); got != 5 {
		t.Fatalf("probe_events count = %d, want 5", got)
	}
	if got := historyTableCount(t, hs, "probe_incidents"); got != 2 {
		t.Fatalf("probe_incidents count = %d, want 2", got)
	}

	events := hs.ProbeEventsFor(historyEntityNode, "node1.prg", fixedNow, historyDefaultDays)
	if len(events) != 2 {
		t.Fatalf("windowed probe events = %+v, want 2", events)
	}

	incidents := hs.ProbeIncidents(fixedNow, historyDefaultDays)
	if len(incidents) != 1 || incidents[0].StartsAt.Before(historyStartDay(fixedNow, historyDefaultDays)) {
		t.Fatalf("windowed probe incidents = %+v, want only current-window incident", incidents)
	}
}

func TestHistoryStoreBatchesProbeEventQueries(t *testing.T) {
	hs, err := openHistoryStore(t.TempDir())
	if err != nil {
		t.Fatalf("open history store: %v", err)
	}

	windowStart := fixedNow.AddDate(0, 0, -30)
	node1 := ProbeTarget{
		EntityKind:  historyEntityNode,
		EntityID:    "node1.prg",
		EntityLabel: "node1.prg",
		Method:      "Ping",
	}
	node2 := ProbeTarget{
		EntityKind:  historyEntityNode,
		EntityID:    "node2.prg",
		EntityLabel: "node2.prg",
		Method:      "Ping",
	}
	records := []struct {
		target  ProbeTarget
		status  string
		message string
		at      time.Time
	}{
		{node1, historyProbeStateOperational, "responding", windowStart.Add(-time.Hour)},
		{node1, historyProbeStateDown, "not responding", windowStart.Add(time.Hour)},
		{node1, historyProbeStateOperational, "responding", windowStart.Add(2 * time.Hour)},
		{node2, historyProbeStateOperational, "responding", windowStart.Add(-2 * time.Hour)},
	}
	for _, record := range records {
		if err := hs.RecordProbeStatus(record.target, record.status, record.message, record.at); err != nil {
			t.Fatalf("record probe status: %v", err)
		}
	}

	targets := []historyEntityInfo{
		{Kind: historyEntityNode, ID: "node1.prg", Label: "node1.prg"},
		{Kind: historyEntityNode, ID: "node2.prg", Label: "node2.prg"},
	}
	availabilityEvents := hs.ProbeEventsForAvailabilityTargets(targets, windowStart, fixedNow)
	node1Events := availabilityEvents[historyKey(historyEntityNode, "node1.prg")]
	if len(node1Events) != 3 ||
		node1Events[0].Status != historyProbeStateOperational ||
		node1Events[1].Status != historyProbeStateDown ||
		node1Events[2].Status != historyProbeStateOperational {
		t.Fatalf("node1 availability events = %+v", node1Events)
	}
	node2Events := availabilityEvents[historyKey(historyEntityNode, "node2.prg")]
	if len(node2Events) != 1 || node2Events[0].Status != historyProbeStateOperational {
		t.Fatalf("node2 availability events = %+v", node2Events)
	}

	recentEvents := hs.ProbeEventsForTargets(targets, fixedNow, historyDefaultDays)
	if len(recentEvents) != 4 || !recentEvents[0].ChangedAt.Equal(windowStart.Add(2*time.Hour)) {
		t.Fatalf("batched recent events = %+v", recentEvents)
	}
}

func TestHistoryStorePaginatesProbeLogForEntity(t *testing.T) {
	hs, err := openHistoryStore(t.TempDir())
	if err != nil {
		t.Fatalf("open history store: %v", err)
	}

	target := ProbeTarget{
		EntityKind:  historyEntityNode,
		EntityID:    "node1.prg",
		EntityLabel: "node1.prg",
		Method:      "Ping",
	}
	records := []struct {
		status string
		at     time.Time
	}{
		{historyProbeStateOperational, fixedNow.Add(-4 * time.Hour)},
		{historyProbeStateDown, fixedNow.Add(-3 * time.Hour)},
		{historyProbeStateOperational, fixedNow.Add(-2 * time.Hour)},
		{historyProbeStateDegraded, fixedNow.Add(-time.Hour)},
	}
	for _, record := range records {
		if err := hs.RecordProbeStatus(target, record.status, record.status, record.at); err != nil {
			t.Fatalf("record probe status: %v", err)
		}
	}

	page := hs.ProbeLogFor(historyEntityNode, "node1.prg", fixedNow, historyDefaultDays, 2, 0)
	if page.Total != 4 || len(page.Events) != 2 {
		t.Fatalf("first page = %+v, want total 4 and 2 events", page)
	}
	if page.Events[0].Status != historyProbeStateDegraded || !page.Events[0].EndsAt.IsZero() {
		t.Fatalf("latest event = %+v, want degraded open event", page.Events[0])
	}
	if page.Events[1].Status != historyProbeStateOperational || !page.Events[1].EndsAt.Equal(records[3].at) {
		t.Fatalf("second event = %+v, want operational ending at next event", page.Events[1])
	}

	page = hs.ProbeLogFor(historyEntityNode, "node1.prg", fixedNow, historyDefaultDays, 2, 2)
	if page.Total != 4 || len(page.Events) != 2 {
		t.Fatalf("second page = %+v, want total 4 and 2 events", page)
	}
	if page.Events[0].Status != historyProbeStateDown || !page.Events[0].EndsAt.Equal(records[2].at) {
		t.Fatalf("third event = %+v, want down ending at recovery", page.Events[0])
	}
	if page.Events[1].Status != historyProbeStateOperational || !page.Events[1].EndsAt.Equal(records[1].at) {
		t.Fatalf("fourth event = %+v, want operational ending at failure", page.Events[1])
	}
}

func TestHistoryStorePaginatesProbeLogForTargets(t *testing.T) {
	hs, err := openHistoryStore(t.TempDir())
	if err != nil {
		t.Fatalf("open history store: %v", err)
	}

	node1 := ProbeTarget{
		EntityKind:  historyEntityNode,
		EntityID:    "node1.prg",
		EntityLabel: "node1.prg",
		Method:      "Ping",
	}
	node2 := ProbeTarget{
		EntityKind:  historyEntityNode,
		EntityID:    "node2.prg",
		EntityLabel: "node2.prg",
		Method:      "Ping",
	}
	other := ProbeTarget{
		EntityKind:  historyEntityDnsResolver,
		EntityID:    "resolver-prg",
		EntityLabel: "resolver-prg",
		Method:      "Lookup",
	}
	records := []struct {
		target ProbeTarget
		status string
		at     time.Time
	}{
		{node1, historyProbeStateOperational, fixedNow.Add(-4 * time.Hour)},
		{node1, historyProbeStateDown, fixedNow.Add(-3 * time.Hour)},
		{node1, historyProbeStateOperational, fixedNow.Add(-2 * time.Hour)},
		{node2, historyProbeStateOperational, fixedNow.Add(-90 * time.Minute)},
		{node1, historyProbeStateDegraded, fixedNow.Add(-time.Hour)},
		{other, historyProbeStateOperational, fixedNow.Add(-30 * time.Minute)},
	}
	for _, record := range records {
		if err := hs.RecordProbeStatus(record.target, record.status, record.status, record.at); err != nil {
			t.Fatalf("record probe status: %v", err)
		}
	}

	targets := []historyEntityInfo{
		{Kind: historyEntityNode, ID: "node1.prg", Label: "node1.prg"},
		{Kind: historyEntityNode, ID: "node2.prg", Label: "node2.prg"},
	}
	page := hs.ProbeLogForTargets(targets, fixedNow, historyDefaultDays, 3, 0)
	if page.Total != 5 || len(page.Events) != 3 {
		t.Fatalf("group first page = %+v, want total 5 and 3 events", page)
	}
	if page.Events[0].EntityID != "node1.prg" || page.Events[0].Status != historyProbeStateDegraded {
		t.Fatalf("group latest event = %+v, want node1 degraded", page.Events[0])
	}
	if page.Events[1].EntityID != "node2.prg" || page.Events[1].Status != historyProbeStateOperational {
		t.Fatalf("group second event = %+v, want node2 operational", page.Events[1])
	}
	if !page.Events[2].EndsAt.Equal(records[4].at) {
		t.Fatalf("group third event = %+v, want node1 event ending at next node1 change", page.Events[2])
	}

	page = hs.ProbeLogForTargets(targets, fixedNow, historyDefaultDays, 3, 3)
	if page.Total != 5 || len(page.Events) != 2 {
		t.Fatalf("group second page = %+v, want total 5 and 2 events", page)
	}
	if page.Events[0].EntityID != "node1.prg" || page.Events[0].Status != historyProbeStateDown {
		t.Fatalf("group page 2 first event = %+v, want node1 down", page.Events[0])
	}
}

func TestHistoryStorePersistsOutages(t *testing.T) {
	dir := t.TempDir()
	hs, err := openHistoryStore(dir)
	if err != nil {
		t.Fatalf("open history store: %v", err)
	}

	report := &OutageReport{
		Id:        1001,
		BeginsAt:  fixedNow.Add(-24 * time.Hour),
		Duration:  30 * time.Minute,
		Type:      "outage",
		State:     "resolved",
		EnSummary: "Power failure",
		AffectedEntities: []OutageEntity{
			{Name: "Node", Id: 101, Label: "Node node1.prg"},
		},
	}
	oldReport := testHistoryOutage(900, fixedNow.AddDate(0, 0, -200), "Old outage", []OutageEntity{
		{Name: "Node", Id: 102, Label: "Node node2.prg"},
	})

	if err := hs.ReplaceOutages([]*OutageReport{oldReport, report}, fixedNow); err != nil {
		t.Fatalf("replace outages: %v", err)
	}

	updated := testHistoryOutage(1001, fixedNow.Add(-24*time.Hour), "Updated power failure", []OutageEntity{
		{Name: "Node", Id: 103, Label: "Node node3.prg"},
	})
	newReport := testHistoryOutage(1002, fixedNow.Add(-2*time.Hour), "New outage", []OutageEntity{
		{Name: "Location", Id: 3, Label: "Location Praha"},
	})
	if err := hs.ReplaceOutages([]*OutageReport{updated, newReport}, fixedNow); err != nil {
		t.Fatalf("replace outages: %v", err)
	}

	reopened, err := openHistoryStore(dir)
	if err != nil {
		t.Fatalf("reopen history store: %v", err)
	}

	reports := reopened.OutageReports()
	if len(reports) != 3 {
		t.Fatalf("stored reports = %+v", reports)
	}
	if got := historyReportByID(reports, 900); got == nil || got.AffectedEntities[0].Label != "Node node2.prg" {
		t.Fatalf("old report = %+v, want retained", got)
	}
	if got := historyReportByID(reports, 1001); got == nil || got.EnSummary != "Updated power failure" || got.Duration != 30*time.Minute || got.AffectedEntities[0].Label != "Node node3.prg" {
		t.Fatalf("updated report = %+v", got)
	}
	if got := historyReportByID(reports, 1002); got == nil || got.AffectedEntities[0].Label != "Location Praha" {
		t.Fatalf("new report = %+v", got)
	}
}

func TestHistoryStoreRecordsEntitySnapshots(t *testing.T) {
	hs, err := openHistoryStore(t.TempDir())
	if err != nil {
		t.Fatalf("open history store: %v", err)
	}

	if err := hs.RecordEntitySnapshots([]HistoryEntitySnapshot{
		{
			EntityKind:         historyEntityNode,
			EntityID:           "node9.stg",
			EntityLabel:        "node9.stg",
			NodeID:             450,
			GroupKind:          historyGroupLocation,
			GroupID:            3,
			GroupLabel:         "Praha",
			VpsAdminLocationID: 7,
			LastSeen:           fixedNow.Add(-time.Hour),
		},
	}); err != nil {
		t.Fatalf("record entity snapshots: %v", err)
	}

	if err := hs.RecordEntitySnapshots([]HistoryEntitySnapshot{
		{
			EntityKind:         historyEntityNode,
			EntityID:           "node9.stg",
			EntityLabel:        "node9.stg",
			NodeID:             450,
			GroupKind:          historyGroupLocation,
			GroupID:            3,
			GroupLabel:         "Praha",
			VpsAdminLocationID: 7,
			LastSeen:           fixedNow,
		},
	}); err != nil {
		t.Fatalf("update entity snapshots: %v", err)
	}

	snapshots := hs.EntitySnapshots()
	if len(snapshots) != 1 {
		t.Fatalf("entity snapshots = %+v, want one snapshot", snapshots)
	}

	got := snapshots[0]
	if got.EntityKind != historyEntityNode ||
		got.EntityID != "node9.stg" ||
		got.EntityLabel != "node9.stg" ||
		got.NodeID != 450 ||
		got.GroupKind != historyGroupLocation ||
		got.GroupID != 3 ||
		got.GroupLabel != "Praha" ||
		got.VpsAdminLocationID != 7 ||
		!got.LastSeen.Equal(fixedNow) {
		t.Fatalf("entity snapshot = %+v", got)
	}
}

func TestHistoryViewsApplyOutageAndProbeSeverity(t *testing.T) {
	_, st, _ := newTestApplication(t)
	setOperationalFixture(st)

	nodeOutage := &OutageReport{
		Id:        1001,
		BeginsAt:  fixedNow.Add(-24 * time.Hour),
		Duration:  30 * time.Minute,
		Type:      "outage",
		State:     "resolved",
		EnSummary: "Power failure",
		AffectedEntities: []OutageEntity{
			{Name: "Node", Id: 101, Label: "Node node1.prg"},
		},
	}
	locationOutage := &OutageReport{
		Id:        1002,
		BeginsAt:  fixedNow.Add(-48 * time.Hour),
		Duration:  30 * time.Minute,
		Type:      "outage",
		State:     "resolved",
		EnSummary: "Location power failure",
		AffectedEntities: []OutageEntity{
			{Name: "Location", Id: 3, Label: "Praha"},
		},
	}
	if err := st.History.ReplaceOutages([]*OutageReport{nodeOutage, locationOutage}, fixedNow); err != nil {
		t.Fatalf("replace outages: %v", err)
	}

	nodeTarget := ProbeTarget{
		EntityKind:  historyEntityNode,
		EntityID:    "node2.prg",
		EntityLabel: "node2.prg",
		Method:      "Ping",
	}
	if err := st.History.RecordProbeStatus(nodeTarget, historyProbeStateDown, "not responding", fixedNow.Add(-10*time.Minute)); err != nil {
		t.Fatalf("record probe status: %v", err)
	}
	if err := st.History.RecordProbeStatus(ProbeTarget{
		EntityKind:  historyEntityDnsResolver,
		EntityID:    "resolver-prg",
		EntityLabel: "resolver-prg",
		Method:      "DNS",
	}, historyProbeStateDown, "lookup failed", fixedNow.Add(-10*time.Minute)); err != nil {
		t.Fatalf("record resolver probe status: %v", err)
	}
	if err := st.History.RecordProbeStatus(ProbeTarget{
		EntityKind:  historyEntityVpsAdmin,
		EntityID:    "api",
		EntityLabel: "vpsAdmin API",
		Method:      "HTTP",
	}, historyProbeStateDown, "HTTP 500", fixedNow.Add(-10*time.Minute)); err != nil {
		t.Fatalf("record vpsAdmin probe status: %v", err)
	}
	if err := st.History.RecordProbeStatus(ProbeTarget{
		EntityKind:  historyEntityWebService,
		EntityID:    "vpsfree.cz",
		EntityLabel: "vpsfree.cz",
		Method:      "HTTP",
	}, historyProbeStateDown, "HTTP 500", fixedNow.Add(-10*time.Minute)); err != nil {
		t.Fatalf("record service probe status: %v", err)
	}
	if err := st.History.RecordProbeStatus(ProbeTarget{
		EntityKind:  historyEntityNameServer,
		EntityID:    "ns1.vpsfree.cz",
		EntityLabel: "ns1.vpsfree.cz",
		Method:      "DNS",
	}, historyProbeStateDown, "lookup failed", fixedNow.Add(-10*time.Minute)); err != nil {
		t.Fatalf("record nameserver probe status: %v", err)
	}

	view := createStatusView(st, fixedNow)
	node1 := view.HistoryFor(historyEntityNode, "node1.prg")
	node2 := view.HistoryFor(historyEntityNode, "node2.prg")
	praha := view.Locations[0].History

	if got := historyDayState(node1, fixedNow.Add(-24*time.Hour)); got != historySeverityOutage {
		t.Fatalf("node1 outage day = %q, want outage", got)
	}
	if got := historyDayState(node2, fixedNow); got != historySeverityMaintenance {
		t.Fatalf("node2 probe day = %q, want maintenance", got)
	}
	if len(node1.Lanes) != 0 {
		t.Fatalf("per-entity node history lanes = %+v, want none", node1.Lanes)
	}
	if incident, ok := historyDayIncident(node1, fixedNow.Add(-24*time.Hour), "Outage: Power failure"); !ok {
		t.Fatalf("node1 outage incidents = %+v, want outage incident", historyDayIncidents(node1, fixedNow.Add(-24*time.Hour)))
	} else if incident.StartLabel != "Started: "+nodeOutage.BeginsAt.Local().Format(historyIncidentTimeFormat) || incident.DurationLabel != "Expected duration: 30 min" {
		t.Fatalf("node1 outage incident = %+v, want start and expected duration", incident)
	}
	if incident, ok := historyDayIncident(node2, fixedNow, "Probe: node2.prg Ping not responding"); !ok {
		t.Fatalf("node2 probe incidents = %+v, want probe incident", historyDayIncidents(node2, fixedNow))
	} else if incident.StartLabel != "Started: "+fixedNow.Add(-10*time.Minute).Local().Format(historyIncidentTimeFormat) || incident.DurationLabel != "Observed duration: 10 min so far" {
		t.Fatalf("node2 probe incident = %+v, want start and observed duration", incident)
	}

	if got := historyDayState(praha, fixedNow.Add(-24*time.Hour)); got != historySeverityOutage {
		t.Fatalf("Praha outage day = %q, want outage", got)
	}
	if got := historyDayLaneState(praha, fixedNow.Add(-24*time.Hour), historyKey(historyEntityNode, "node1.prg")); got != historySeverityOutage {
		t.Fatalf("Praha node1 outage lane = %q, want outage", got)
	}
	if !historyDayHasIncident(praha, fixedNow.Add(-24*time.Hour), "node1.prg: Outage: Power failure") {
		t.Fatalf("Praha outage popover incidents = %+v, want node label", historyDayIncidents(praha, fixedNow.Add(-24*time.Hour)))
	}
	if got := historyDayLaneState(praha, fixedNow.Add(-24*time.Hour), historyKey(historyEntityNode, "node2.prg")); got != historySeverityOperational {
		t.Fatalf("Praha node2 outage lane = %q, want operational", got)
	}
	if got := historyDayState(praha, fixedNow); got != historySeverityMaintenance {
		t.Fatalf("Praha probe day = %q, want maintenance", got)
	}
	if got := historyDayLaneState(praha, fixedNow, historyKey(historyEntityNode, "node2.prg")); got != historySeverityMaintenance {
		t.Fatalf("Praha node2 probe lane = %q, want maintenance", got)
	}
	if got := historyDayLaneState(praha, fixedNow, historyKey(historyEntityDnsResolver, "resolver-prg")); got != historySeverityMaintenance {
		t.Fatalf("Praha resolver probe lane = %q, want maintenance", got)
	}
	if got := historyDayLaneState(praha, fixedNow, historyKey(historyEntityNode, "node1.prg")); got != historySeverityOperational {
		t.Fatalf("Praha node1 probe lane = %q, want operational", got)
	}
	if got := historyDayLaneState(praha, fixedNow.Add(-48*time.Hour), historyKey(historyEntityNode, "node1.prg")); got != historySeverityOutage {
		t.Fatalf("Praha location node1 lane = %q, want outage", got)
	}
	if got := historyDayLaneState(praha, fixedNow.Add(-48*time.Hour), historyKey(historyEntityNode, "node2.prg")); got != historySeverityOutage {
		t.Fatalf("Praha location node2 lane = %q, want outage", got)
	}
	if got := historyDayLaneState(praha, fixedNow.Add(-48*time.Hour), historyKey(historyEntityDnsResolver, "resolver-prg")); got != historySeverityOperational {
		t.Fatalf("Praha location resolver lane = %q, want operational", got)
	}

	if got := historyDayState(view.VpsAdmin.History, fixedNow); got != historySeverityMaintenance {
		t.Fatalf("vpsAdmin probe day = %q, want maintenance", got)
	}
	if got := historyDayLaneState(view.VpsAdmin.History, fixedNow, historyKey(historyEntityVpsAdmin, "api")); got != historySeverityMaintenance {
		t.Fatalf("vpsAdmin API lane = %q, want maintenance", got)
	}
	if got := historyDayLaneState(view.VpsAdmin.History, fixedNow, historyKey(historyEntityVpsAdmin, "webui")); got != historySeverityOperational {
		t.Fatalf("vpsAdmin web UI lane = %q, want operational", got)
	}

	if got := historyDayState(view.Services.History, fixedNow); got != historySeverityMaintenance {
		t.Fatalf("Services probe day = %q, want maintenance", got)
	}
	if got := historyDayLaneState(view.Services.History, fixedNow, historyKey(historyEntityWebService, "vpsfree.cz")); got != historySeverityMaintenance {
		t.Fatalf("Services web lane = %q, want maintenance", got)
	}
	if got := historyDayLaneState(view.Services.History, fixedNow, historyKey(historyEntityNameServer, "ns1.vpsfree.cz")); got != historySeverityMaintenance {
		t.Fatalf("Services nameserver lane = %q, want maintenance", got)
	}
	if got := historyDayLaneState(view.Services.History, fixedNow, historyKey(historyEntityWebService, "kb.vpsfree.cz")); got != historySeverityOperational {
		t.Fatalf("Services kb lane = %q, want operational", got)
	}
}

func TestHistoryViewsHideProbeIncidentsCoveredByReports(t *testing.T) {
	_, st, _ := newTestApplication(t)
	setOperationalFixture(st)

	nodeOutage := testHistoryOutage(4001, fixedNow.Add(-20*time.Minute), "Node outage", []OutageEntity{
		{Name: "Node", Id: 101, Label: "Node node1.prg"},
	})
	serviceMaintenance := testHistoryOutage(4002, fixedNow.Add(-70*time.Minute), "Service maintenance", []OutageEntity{
		{Name: "Web service", Label: "vpsfree.cz"},
	})
	serviceMaintenance.Type = "maintenance"
	nameserverOutage := testHistoryOutage(4003, fixedNow.Add(-80*time.Minute), "Nameserver outage", []OutageEntity{
		{Name: "Name server", Label: "ns1.vpsfree.cz"},
	})
	if err := st.History.ReplaceOutages([]*OutageReport{nodeOutage, serviceMaintenance, nameserverOutage}, fixedNow); err != nil {
		t.Fatalf("replace outages: %v", err)
	}

	recordPromotedProbeIncident(t, st, ProbeTarget{
		EntityKind:  historyEntityNode,
		EntityID:    "node1.prg",
		EntityLabel: "node1.prg",
		Method:      "Ping",
	}, "not responding", fixedNow.Add(-10*time.Minute))
	recordPromotedProbeIncident(t, st, ProbeTarget{
		EntityKind:  historyEntityWebService,
		EntityID:    "vpsfree.cz",
		EntityLabel: "vpsfree.cz",
		Method:      "HTTP",
	}, "HTTP 500", fixedNow.Add(-10*time.Minute))
	recordPromotedProbeIncident(t, st, ProbeTarget{
		EntityKind:  historyEntityNameServer,
		EntityID:    "ns1.vpsfree.cz",
		EntityLabel: "ns1.vpsfree.cz",
		Method:      "DNS",
	}, "lookup failed", fixedNow.Add(-10*time.Minute))
	recordPromotedProbeIncident(t, st, ProbeTarget{
		EntityKind:  historyEntityWebService,
		EntityID:    "kb.vpsfree.cz",
		EntityLabel: "kb.vpsfree.cz",
		Method:      "HTTP",
	}, "HTTP 500", fixedNow.Add(-10*time.Minute))

	view := createStatusView(st, fixedNow)
	nodeHistory := view.HistoryFor(historyEntityNode, "node1.prg")
	serviceHistory := view.HistoryFor(historyEntityWebService, "vpsfree.cz")
	nameserverHistory := view.HistoryFor(historyEntityNameServer, "ns1.vpsfree.cz")
	kbHistory := view.HistoryFor(historyEntityWebService, "kb.vpsfree.cz")

	if historyDayHasIncident(nodeHistory, fixedNow, "Probe: node1.prg Ping not responding") {
		t.Fatalf("covered node probe incidents = %+v, want probe hidden", historyDayIncidents(nodeHistory, fixedNow))
	}
	if !historyDayHasIncident(nodeHistory, fixedNow, "Outage: Node outage") {
		t.Fatalf("node incidents = %+v, want outage retained", historyDayIncidents(nodeHistory, fixedNow))
	}
	if historyDayHasIncident(serviceHistory, fixedNow, "Probe: vpsfree.cz HTTP HTTP 500") {
		t.Fatalf("grace-covered service probe incidents = %+v, want probe hidden", historyDayIncidents(serviceHistory, fixedNow))
	}
	if !historyDayHasIncident(serviceHistory, fixedNow, "Maintenance: Service maintenance") {
		t.Fatalf("service incidents = %+v, want maintenance retained", historyDayIncidents(serviceHistory, fixedNow))
	}
	if !historyDayHasIncident(nameserverHistory, fixedNow, "Probe: ns1.vpsfree.cz DNS lookup failed") {
		t.Fatalf("outside-grace nameserver incidents = %+v, want probe retained", historyDayIncidents(nameserverHistory, fixedNow))
	}
	if !historyDayHasIncident(kbHistory, fixedNow, "Probe: kb.vpsfree.cz HTTP HTTP 500") {
		t.Fatalf("uncovered kb incidents = %+v, want probe retained", historyDayIncidents(kbHistory, fixedNow))
	}

	praha := view.Locations[0].History
	if historyDayHasIncident(praha, fixedNow, "Probe: node1.prg Ping not responding") {
		t.Fatalf("Praha incidents = %+v, want covered node probe hidden", historyDayIncidents(praha, fixedNow))
	}
	if !historyDayHasIncident(praha, fixedNow, "node1.prg: Outage: Node outage") {
		t.Fatalf("Praha incidents = %+v, want node outage retained", historyDayIncidents(praha, fixedNow))
	}

	services := view.Services.History
	if historyDayHasIncident(services, fixedNow, "Probe: vpsfree.cz HTTP HTTP 500") {
		t.Fatalf("Services incidents = %+v, want grace-covered service probe hidden", historyDayIncidents(services, fixedNow))
	}
	if !historyDayHasIncident(services, fixedNow, "Probe: ns1.vpsfree.cz DNS lookup failed") {
		t.Fatalf("Services incidents = %+v, want outside-grace nameserver probe retained", historyDayIncidents(services, fixedNow))
	}
	if !historyDayHasIncident(services, fixedNow, "Probe: kb.vpsfree.cz HTTP HTTP 500") {
		t.Fatalf("Services incidents = %+v, want unrelated service probe retained", historyDayIncidents(services, fixedNow))
	}
}

func TestHistoryViewsUseConfiguredHistoryDays(t *testing.T) {
	_, st, _ := newTestApplication(t)
	setOperationalFixture(st)
	st.HistoryDays = 7

	view := createStatusView(st, fixedNow)
	history := view.HistoryFor(historyEntityNode, "node1.prg")

	if len(history.Days) != 7 {
		t.Fatalf("history days = %d, want 7", len(history.Days))
	}
	if got, want := history.Days[0].Date, localDay(fixedNow).AddDate(0, 0, -6).Format("2006-01-02"); got != want {
		t.Fatalf("history start day = %q, want %q", got, want)
	}
	if got, want := history.Days[6].Date, localDay(fixedNow).Format("2006-01-02"); got != want {
		t.Fatalf("history end day = %q, want %q", got, want)
	}
}

func TestHistoryViewsMapCollapsedLocationsEnvironmentsAndCluster(t *testing.T) {
	_, st, _ := newTestApplication(t)
	addHistoryTestNode(st, "Praha", 400, "node1.stg", 7)
	addHistoryTestNode(st, "Praha", 401, "node2.stg", 7)
	addHistoryTestNode(st, "Praha", 402, "backuper2.prg", 6)
	addHistoryTestNode(st, "Praha", 403, "node1.pgnd", 5)
	setOperationalFixture(st)

	st.GlobalNodeMap["node1.stg"].LocationId = 7
	st.GlobalNodeMap["node2.stg"].LocationId = 7
	st.GlobalNodeMap["backuper2.prg"].LocationId = 6
	st.GlobalNodeMap["node1.pgnd"].LocationId = 5
	st.VpsAdminLocations = map[int64]VpsAdminLocation{
		3: {Id: 3, Label: "Praha", EnvironmentId: 1, EnvironmentLabel: "Production"},
		4: {Id: 4, Label: "Brno", EnvironmentId: 1, EnvironmentLabel: "Production"},
		5: {Id: 5, Label: "Playground", EnvironmentId: 2, EnvironmentLabel: "Playground"},
		6: {Id: 6, Label: "Praha Storage", EnvironmentId: 3, EnvironmentLabel: "Praha storage"},
		7: {Id: 7, Label: "Staging", EnvironmentId: 5, EnvironmentLabel: "Staging"},
	}

	stagingLocationDay := fixedNow.Add(-24 * time.Hour)
	storageLocationDay := fixedNow.Add(-48 * time.Hour)
	stagingEnvironmentDay := fixedNow.Add(-72 * time.Hour)
	productionEnvironmentDay := fixedNow.Add(-96 * time.Hour)
	clusterDay := fixedNow.Add(-120 * time.Hour)

	reports := []*OutageReport{
		testHistoryOutage(2001, stagingLocationDay, "Staging location outage", []OutageEntity{
			{Name: "Location", Id: 7, Label: "Location Staging"},
		}),
		testHistoryOutage(2002, storageLocationDay, "Storage location outage", []OutageEntity{
			{Name: "Location", Id: 6, Label: "Location Praha Storage"},
		}),
		testHistoryOutage(2003, stagingEnvironmentDay, "Staging environment outage", []OutageEntity{
			{Name: "Environment", Id: 5, Label: "Environment Staging"},
		}),
		testHistoryOutage(2004, productionEnvironmentDay, "Production environment outage", []OutageEntity{
			{Name: "Environment", Id: 1, Label: "Environment Production"},
		}),
		testHistoryOutage(2005, clusterDay, "Cluster outage", []OutageEntity{
			{Name: "Cluster", Label: "All systems within the cluster"},
		}),
	}

	if err := st.History.ReplaceOutages(reports, fixedNow); err != nil {
		t.Fatalf("replace outages: %v", err)
	}

	view := createStatusView(st, fixedNow)
	praha := view.Locations[0].History
	brno := view.Locations[1].History

	assertHistoryLaneState(t, praha, stagingLocationDay, historyEntityNode, "node1.stg", historySeverityOutage)
	assertHistoryLaneState(t, praha, stagingLocationDay, historyEntityNode, "node2.stg", historySeverityOutage)
	assertHistoryLaneState(t, praha, stagingLocationDay, historyEntityNode, "node1.prg", historySeverityOperational)
	assertHistoryLaneState(t, praha, stagingLocationDay, historyEntityNode, "backuper2.prg", historySeverityOperational)
	assertHistoryLaneState(t, brno, stagingLocationDay, historyEntityNode, "node1.brq", historySeverityOperational)

	assertHistoryLaneState(t, praha, storageLocationDay, historyEntityNode, "backuper2.prg", historySeverityOutage)
	assertHistoryLaneState(t, praha, storageLocationDay, historyEntityNode, "node1.stg", historySeverityOperational)
	assertHistoryLaneState(t, praha, storageLocationDay, historyEntityNode, "node1.prg", historySeverityOperational)

	assertHistoryLaneState(t, praha, stagingEnvironmentDay, historyEntityNode, "node1.stg", historySeverityOutage)
	assertHistoryLaneState(t, praha, stagingEnvironmentDay, historyEntityNode, "node2.stg", historySeverityOutage)
	assertHistoryLaneState(t, praha, stagingEnvironmentDay, historyEntityNode, "node1.prg", historySeverityOperational)
	assertHistoryLaneState(t, praha, stagingEnvironmentDay, historyEntityNode, "backuper2.prg", historySeverityOperational)
	assertHistoryLaneState(t, praha, stagingEnvironmentDay, historyEntityNode, "node1.pgnd", historySeverityOperational)

	assertHistoryLaneState(t, praha, productionEnvironmentDay, historyEntityNode, "node1.prg", historySeverityOutage)
	assertHistoryLaneState(t, praha, productionEnvironmentDay, historyEntityNode, "node2.prg", historySeverityOutage)
	assertHistoryLaneState(t, brno, productionEnvironmentDay, historyEntityNode, "node1.brq", historySeverityOutage)
	assertHistoryLaneState(t, praha, productionEnvironmentDay, historyEntityNode, "node1.stg", historySeverityOperational)
	assertHistoryLaneState(t, praha, productionEnvironmentDay, historyEntityNode, "backuper2.prg", historySeverityOperational)
	assertHistoryLaneState(t, praha, productionEnvironmentDay, historyEntityNode, "node1.pgnd", historySeverityOperational)

	assertHistoryLaneState(t, praha, clusterDay, historyEntityNode, "node1.prg", historySeverityOutage)
	assertHistoryLaneState(t, praha, clusterDay, historyEntityNode, "node1.stg", historySeverityOutage)
	assertHistoryLaneState(t, praha, clusterDay, historyEntityNode, "backuper2.prg", historySeverityOutage)
	assertHistoryLaneState(t, praha, clusterDay, historyEntityNode, "node1.pgnd", historySeverityOutage)
	assertHistoryLaneState(t, brno, clusterDay, historyEntityNode, "node1.brq", historySeverityOutage)
	assertHistoryLaneState(t, praha, clusterDay, historyEntityDnsResolver, "resolver-prg", historySeverityOperational)
	if got := historyDayState(view.HistoryFor(historyEntityDnsResolver, "resolver-prg"), clusterDay); got != historySeverityOperational {
		t.Fatalf("resolver cluster day = %q, want operational", got)
	}
}

func TestHistoryViewsIncludeRemovedNodeOutagesInGroupLane(t *testing.T) {
	_, st, _ := newTestApplication(t)
	setOperationalFixture(st)
	addHistoryTestNode(st, "Praha", 450, "node9.stg", 7)
	st.VpsAdminLocations = map[int64]VpsAdminLocation{
		3: {Id: 3, Label: "Praha", EnvironmentId: 1, EnvironmentLabel: "Production"},
		4: {Id: 4, Label: "Brno", EnvironmentId: 1, EnvironmentLabel: "Production"},
		7: {Id: 7, Label: "Staging", EnvironmentId: 5, EnvironmentLabel: "Staging"},
	}

	if err := recordConfiguredEntitySnapshots(st, fixedNow.Add(-96*time.Hour)); err != nil {
		t.Fatalf("record snapshots: %v", err)
	}
	removeHistoryTestNode(st, "Praha", "node9.stg")

	nodeDay := fixedNow.Add(-24 * time.Hour)
	locationDay := fixedNow.Add(-48 * time.Hour)
	environmentDay := fixedNow.Add(-72 * time.Hour)
	reports := []*OutageReport{
		testHistoryOutage(3001, nodeDay, "Removed node outage", []OutageEntity{
			{Name: "Node", Id: 450, Label: "Node node9.stg"},
		}),
		testHistoryOutage(3002, locationDay, "Staging location outage", []OutageEntity{
			{Name: "Location", Id: 7, Label: "Location Staging"},
		}),
		testHistoryOutage(3003, environmentDay, "Staging environment outage", []OutageEntity{
			{Name: "Environment", Id: 5, Label: "Environment Staging"},
		}),
	}
	if err := st.History.ReplaceOutages(reports, fixedNow); err != nil {
		t.Fatalf("replace outages: %v", err)
	}

	view := createStatusView(st, fixedNow)
	praha := view.Locations[0].History

	if got, want := historyLaneLabels(praha), []string{"node1.prg", "node2.prg", "node9.stg (removed)", "resolver-prg"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Praha history lanes = %+v, want %+v", got, want)
	}
	assertHistoryLaneState(t, praha, nodeDay, historyEntityNode, "node9.stg", historySeverityOutage)
	assertHistoryLaneState(t, praha, locationDay, historyEntityNode, "node9.stg", historySeverityOutage)
	assertHistoryLaneState(t, praha, environmentDay, historyEntityNode, "node9.stg", historySeverityOutage)
	assertHistoryLaneState(t, praha, locationDay, historyEntityNode, "node1.prg", historySeverityOperational)
	assertHistoryLaneState(t, praha, environmentDay, historyEntityNode, "node2.prg", historySeverityOperational)
	if !historyDayHasIncident(praha, nodeDay, "node9.stg (removed): Outage: Removed node outage") {
		t.Fatalf("removed node incidents = %+v, want removed-node label", historyDayIncidents(praha, nodeDay))
	}
	if got := view.HistoryFor(historyEntityNode, "node9.stg"); len(got.Days) != 0 {
		t.Fatalf("removed node detail history days = %d, want none", len(got.Days))
	}
}

func TestHistoryViewsIncludeRemovedNodeProbeIncidentInGroupLane(t *testing.T) {
	_, st, _ := newTestApplication(t)
	setOperationalFixture(st)

	target := ProbeTarget{
		EntityKind:  historyEntityNode,
		EntityID:    "node9.brq",
		EntityLabel: "node9.brq",
		Method:      "Ping",
	}
	if err := st.History.RecordProbeStatus(target, historyProbeStateDown, "not responding", fixedNow.Add(-10*time.Minute)); err != nil {
		t.Fatalf("record initial probe failure: %v", err)
	}
	if err := st.History.RecordProbeStatus(target, historyProbeStateDown, "not responding", fixedNow.Add(-4*time.Minute)); err != nil {
		t.Fatalf("record promoted probe failure: %v", err)
	}

	view := createStatusView(st, fixedNow)
	brno := view.Locations[1].History

	if got, want := historyLaneLabels(brno), []string{"node1.brq", "node9.brq (removed)"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Brno history lanes = %+v, want %+v", got, want)
	}
	assertHistoryLaneState(t, brno, fixedNow, historyEntityNode, "node9.brq", historySeverityMaintenance)
	if !historyDayHasIncident(brno, fixedNow, "node9.brq (removed): Probe: node9.brq Ping not responding") {
		t.Fatalf("removed node probe incidents = %+v, want removed-node label", historyDayIncidents(brno, fixedNow))
	}
}

func TestHistoryViewsIgnoreRemovedNodeLaneOutsideHistoryWindow(t *testing.T) {
	_, st, _ := newTestApplication(t)
	setOperationalFixture(st)
	st.HistoryDays = 7
	addHistoryTestNode(st, "Praha", 451, "oldnode.prg", 3)

	if err := recordConfiguredEntitySnapshots(st, fixedNow.AddDate(0, 0, -30)); err != nil {
		t.Fatalf("record snapshots: %v", err)
	}
	removeHistoryTestNode(st, "Praha", "oldnode.prg")

	if err := st.History.ReplaceOutages([]*OutageReport{
		testHistoryOutage(3004, fixedNow.AddDate(0, 0, -30), "Old removed node outage", []OutageEntity{
			{Name: "Node", Id: 451, Label: "Node oldnode.prg"},
		}),
	}, fixedNow); err != nil {
		t.Fatalf("replace outages: %v", err)
	}

	view := createStatusView(st, fixedNow)
	praha := view.Locations[0].History
	if historyBarHasLane(praha, historyKey(historyEntityNode, "oldnode.prg")) {
		t.Fatalf("Praha lanes = %+v, want old removed node omitted", historyLaneLabels(praha))
	}
}

func recordPromotedProbeIncident(t *testing.T, st *Status, target ProbeTarget, message string, startsAt time.Time) {
	t.Helper()

	if err := st.History.RecordProbeStatus(target, historyProbeStateDown, message, startsAt); err != nil {
		t.Fatalf("record initial probe failure: %v", err)
	}
	if err := st.History.RecordProbeStatus(target, historyProbeStateDown, message, startsAt.Add(probeIncidentThreshold)); err != nil {
		t.Fatalf("record promoted probe failure: %v", err)
	}
}

func addHistoryTestNode(st *Status, locLabel string, id int, name string, locationID int) {
	loc := st.LocationMap[locLabel]
	node := &Node{
		Id:         id,
		Name:       name,
		LocationId: locationID,
		IpAddress:  "127.0.0.1",
		Ping: &PingCheck{
			Name:      name,
			IpAddress: "127.0.0.1",
		},
	}
	loc.NodeList = append(loc.NodeList, node)
	loc.NodeMap[name] = node
	st.GlobalNodeMap[name] = node
}

func removeHistoryTestNode(st *Status, locLabel string, name string) {
	loc := st.LocationMap[locLabel]
	if loc == nil {
		return
	}

	delete(loc.NodeMap, name)
	delete(st.GlobalNodeMap, name)
	for i, node := range loc.NodeList {
		if node.Name != name {
			continue
		}

		loc.NodeList = append(loc.NodeList[:i], loc.NodeList[i+1:]...)
		return
	}
}

func testHistoryOutage(id int64, beginsAt time.Time, summary string, entities []OutageEntity) *OutageReport {
	return &OutageReport{
		Id:               id,
		BeginsAt:         beginsAt,
		Duration:         30 * time.Minute,
		Type:             "outage",
		State:            "resolved",
		EnSummary:        summary,
		AffectedEntities: entities,
	}
}

func assertHistoryLaneState(t *testing.T, bar HistoryBarView, day time.Time, kind string, id string, want string) {
	t.Helper()

	if got := historyDayLaneState(bar, day, historyKey(kind, id)); got != want {
		t.Fatalf("%s/%s lane on %s = %q, want %q", kind, id, localDay(day).Format("2006-01-02"), got, want)
	}
}

func historyLaneLabels(bar HistoryBarView) []string {
	ret := make([]string, len(bar.Lanes))
	for i, lane := range bar.Lanes {
		ret[i] = lane.Label
	}
	return ret
}

func historyBarHasLane(bar HistoryBarView, key string) bool {
	for _, lane := range bar.Lanes {
		if lane.Key == key {
			return true
		}
	}
	return false
}

func historyTableCount(t *testing.T, hs *HistoryStore, table string) int {
	t.Helper()

	switch table {
	case "probe_events", "probe_incidents":
	default:
		t.Fatalf("unexpected table %q", table)
	}

	var ret int
	if err := hs.db.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM %s`, table)).Scan(&ret); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return ret
}

func historyReportByID(reports []*OutageReport, id int64) *OutageReport {
	for _, report := range reports {
		if report.Id == id {
			return report
		}
	}
	return nil
}

func historyDayState(bar HistoryBarView, day time.Time) string {
	date := localDay(day).Format("2006-01-02")
	for _, historyDay := range bar.Days {
		if historyDay.Date == date {
			return historyDay.State
		}
	}
	return ""
}

func historyDayLaneState(bar HistoryBarView, day time.Time, key string) string {
	date := localDay(day).Format("2006-01-02")
	for _, historyDay := range bar.Days {
		if historyDay.Date != date {
			continue
		}
		for _, lane := range historyDay.Lanes {
			if lane.Key == key {
				return lane.State
			}
		}
	}
	return ""
}

func historyDayHasIncident(bar HistoryBarView, day time.Time, text string) bool {
	_, ok := historyDayIncident(bar, day, text)
	return ok
}

func historyDayIncident(bar HistoryBarView, day time.Time, text string) (HistoryIncidentView, bool) {
	for _, incident := range historyDayIncidents(bar, day) {
		if incident.Text == text {
			return incident, true
		}
	}
	return HistoryIncidentView{}, false
}

func historyDayIncidents(bar HistoryBarView, day time.Time) []HistoryIncidentView {
	date := localDay(day).Format("2006-01-02")
	for _, historyDay := range bar.Days {
		if historyDay.Date == date {
			return historyDay.Incidents
		}
	}
	return nil
}
