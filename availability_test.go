package main

import (
	"math"
	"testing"
	"time"
)

func TestAvailabilityUsesProbeTransitionsAndCountsDegradedUp(t *testing.T) {
	_, st, _ := newTestApplication(t)
	setOperationalFixture(st)

	windowStart := fixedNow.AddDate(0, 0, -30)
	target := ProbeTarget{
		EntityKind:  historyEntityNode,
		EntityID:    "node1.prg",
		EntityLabel: "node1.prg",
		Method:      "Ping",
	}
	records := []struct {
		status  string
		message string
		at      time.Time
	}{
		{historyProbeStateOperational, "responding", windowStart.Add(-time.Hour)},
		{historyProbeStateDown, "not responding", windowStart.Add(24 * time.Hour)},
		{historyProbeStateDegraded, "20% packet loss", windowStart.Add(48 * time.Hour)},
		{historyProbeStateMaintenance, "under maintenance", windowStart.Add(72 * time.Hour)},
		{historyProbeStateOperational, "responding", windowStart.Add(96 * time.Hour)},
	}
	for _, record := range records {
		if err := st.History.RecordProbeStatus(target, record.status, record.message, record.at); err != nil {
			t.Fatalf("record probe status: %v", err)
		}
	}

	requireReportedAvailabilityPercent(t, st, historyEntityNode, "node1.prg", "30 days", 100)
	requireProbeAvailabilityPercent(t, st, historyEntityNode, "node1.prg", "30 days", 93.333)
}

func TestAvailabilityUsesOnlySelectedProbeMethod(t *testing.T) {
	_, st, _ := newTestApplication(t)
	setOperationalFixture(st)

	windowStart := fixedNow.AddDate(0, 0, -30)
	for _, target := range []ProbeTarget{
		{EntityKind: historyEntityNode, EntityID: "node1.prg", EntityLabel: "node1.prg", Method: "Storage"},
		{EntityKind: historyEntityNode, EntityID: "node1.prg", EntityLabel: "node1.prg", Method: "Ping"},
	} {
		status := historyProbeStateDown
		message := "down"
		if target.Method == "Ping" {
			status = historyProbeStateOperational
			message = "responding"
		}
		if err := st.History.RecordProbeStatus(target, status, message, windowStart.Add(-time.Hour)); err != nil {
			t.Fatalf("record probe status: %v", err)
		}
	}

	requireProbeAvailabilityPercent(t, st, historyEntityNode, "node1.prg", "30 days", 100)
}

func TestAvailabilityReportsOutagesWhenProbeHistoryIsMissing(t *testing.T) {
	_, st, _ := newTestApplication(t)
	setOperationalFixture(st)

	windowStart := fixedNow.AddDate(0, 0, -30)
	reports := []*OutageReport{
		availabilityTestOutage(5001, windowStart.Add(24*time.Hour), 24*time.Hour, []OutageEntity{
			{Name: "Node", Id: 101, Label: "Node node1.prg"},
		}),
	}
	if err := st.History.ReplaceOutages(reports, fixedNow); err != nil {
		t.Fatalf("replace outages: %v", err)
	}

	requireReportedAvailabilityPercent(t, st, historyEntityNode, "node1.prg", "30 days", 96.667)
	requireProbeAvailabilityUnavailable(t, st, historyEntityNode, "node1.prg", "30 days")
}

func TestAvailabilityKeepsReportedAndProbeDowntimeIndependent(t *testing.T) {
	_, st, _ := newTestApplication(t)
	setOperationalFixture(st)

	windowStart := fixedNow.AddDate(0, 0, -30)
	reports := []*OutageReport{
		availabilityTestOutage(5001, windowStart.Add(24*time.Hour), 24*time.Hour, []OutageEntity{
			{Name: "Node", Id: 101, Label: "Node node1.prg"},
		}),
		availabilityTestOutage(5002, windowStart.Add(22*24*time.Hour), 24*time.Hour, []OutageEntity{
			{Name: "Node", Id: 101, Label: "Node node1.prg"},
		}),
	}
	if err := st.History.ReplaceOutages(reports, fixedNow); err != nil {
		t.Fatalf("replace outages: %v", err)
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
		{historyProbeStateOperational, windowStart.Add(-time.Hour)},
		{historyProbeStateDown, windowStart.Add(20 * 24 * time.Hour)},
		{historyProbeStateOperational, windowStart.Add(21 * 24 * time.Hour)},
	}
	for _, record := range records {
		if err := st.History.RecordProbeStatus(target, record.status, record.status, record.at); err != nil {
			t.Fatalf("record probe status: %v", err)
		}
	}

	requireReportedAvailabilityPercent(t, st, historyEntityNode, "node1.prg", "30 days", 93.333)
	requireProbeAvailabilityPercent(t, st, historyEntityNode, "node1.prg", "30 days", 96.667)
}

func TestAvailabilityDoesNotFillProbeGapsFromOutageReports(t *testing.T) {
	_, st, _ := newTestApplication(t)
	setOperationalFixture(st)

	windowStart := fixedNow.AddDate(0, 0, -30)
	reports := []*OutageReport{
		availabilityTestOutage(5001, windowStart.Add(24*time.Hour), 24*time.Hour, []OutageEntity{
			{Name: "Node", Id: 101, Label: "Node node1.prg"},
		}),
	}
	if err := st.History.ReplaceOutages(reports, fixedNow); err != nil {
		t.Fatalf("replace outages: %v", err)
	}

	target := ProbeTarget{
		EntityKind:  historyEntityNode,
		EntityID:    "node1.prg",
		EntityLabel: "node1.prg",
		Method:      "Ping",
	}
	if err := st.History.RecordProbeStatus(target, historyProbeStateOperational, "responding", windowStart.Add(10*24*time.Hour)); err != nil {
		t.Fatalf("record probe status: %v", err)
	}

	requireReportedAvailabilityPercent(t, st, historyEntityNode, "node1.prg", "30 days", 96.667)
	requireProbeAvailabilityUnavailable(t, st, historyEntityNode, "node1.prg", "30 days")
}

func TestAvailabilityIsUnavailableWithoutProbeHistoryOrOutageFallback(t *testing.T) {
	_, st, _ := newTestApplication(t)

	stats := entityAvailability(st, historyEntityNode, "node1.prg", fixedNow)
	if len(stats) == 0 {
		t.Fatal("availability stats are empty")
	}
	if stats[0].Reported.Available || stats[0].Probe.Available {
		t.Fatalf("availability = %+v, want both sources unavailable", stats[0])
	}
}

func TestAvailabilityReportsOutagesForSupportedEntities(t *testing.T) {
	tests := []struct {
		name     string
		kind     string
		id       string
		entities []OutageEntity
	}{
		{
			name: "node",
			kind: historyEntityNode,
			id:   "node1.prg",
			entities: []OutageEntity{
				{Name: "Node", Id: 101, Label: "Node node1.prg"},
			},
		},
		{
			name: "location to node",
			kind: historyEntityNode,
			id:   "node2.prg",
			entities: []OutageEntity{
				{Name: "Location", Id: 3, Label: "Praha"},
			},
		},
		{
			name: "environment to node",
			kind: historyEntityNode,
			id:   "node1.brq",
			entities: []OutageEntity{
				{Name: "Environment", Id: 1, Label: "Production"},
			},
		},
		{
			name: "vpsAdmin service",
			kind: historyEntityVpsAdmin,
			id:   "api",
			entities: []OutageEntity{
				{Name: "Service", Label: "vpsAdmin API"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, st, _ := newTestApplication(t)
			setOperationalFixture(st)
			st.VpsAdminLocations = map[int64]VpsAdminLocation{
				3: {Id: 3, Label: "Praha", EnvironmentId: 1, EnvironmentLabel: "Production"},
				4: {Id: 4, Label: "Brno", EnvironmentId: 1, EnvironmentLabel: "Production"},
			}

			windowStart := fixedNow.AddDate(0, 0, -30)
			report := availabilityTestOutage(6001, windowStart.Add(24*time.Hour), 24*time.Hour, tt.entities)
			if err := st.History.ReplaceOutages([]*OutageReport{report}, fixedNow); err != nil {
				t.Fatalf("replace outages: %v", err)
			}

			requireReportedAvailabilityPercent(t, st, tt.kind, tt.id, "30 days", 96.667)
			requireProbeAvailabilityUnavailable(t, st, tt.kind, tt.id, "30 days")
		})
	}
}

func TestAvailabilityDoesNotReportOutagesForUnsupportedEntities(t *testing.T) {
	tests := []struct {
		name     string
		kind     string
		id       string
		entities []OutageEntity
	}{
		{
			name: "dns resolver",
			kind: historyEntityDnsResolver,
			id:   "resolver-prg",
			entities: []OutageEntity{
				{Name: "DNS Resolver", Label: "resolver-prg"},
			},
		},
		{
			name: "name server",
			kind: historyEntityNameServer,
			id:   "ns1.vpsfree.cz",
			entities: []OutageEntity{
				{Name: "Name Server", Label: "ns1.vpsfree.cz"},
			},
		},
		{
			name: "web service",
			kind: historyEntityWebService,
			id:   "vpsfree.cz",
			entities: []OutageEntity{
				{Name: "Web Service", Label: "vpsfree.cz"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, st, _ := newTestApplication(t)
			setOperationalFixture(st)

			windowStart := fixedNow.AddDate(0, 0, -30)
			report := availabilityTestOutage(6001, windowStart.Add(24*time.Hour), 24*time.Hour, tt.entities)
			if err := st.History.ReplaceOutages([]*OutageReport{report}, fixedNow); err != nil {
				t.Fatalf("replace outages: %v", err)
			}

			method, ok := availabilityProbeMethod(tt.kind)
			if !ok {
				t.Fatalf("no probe method for %s", tt.kind)
			}
			if err := st.History.RecordProbeStatus(ProbeTarget{
				EntityKind:  tt.kind,
				EntityID:    tt.id,
				EntityLabel: tt.id,
				Method:      method,
			}, historyProbeStateOperational, "ok", fixedNow.AddDate(-1, 0, 0).Add(-time.Hour)); err != nil {
				t.Fatalf("record probe status: %v", err)
			}

			requireReportedAvailabilityUnavailable(t, st, tt.kind, tt.id, "30 days")
			requireProbeAvailabilityPercent(t, st, tt.kind, tt.id, "30 days", 100)
		})
	}
}

func requireReportedAvailabilityPercent(t *testing.T, st *Status, kind string, id string, label string, want float64) {
	t.Helper()

	stat := availabilityStat(t, st, kind, id, label)
	requireAvailabilityMetricPercent(t, stat.Reported, label+" reported", want)
}

func requireReportedAvailabilityUnavailable(t *testing.T, st *Status, kind string, id string, label string) {
	t.Helper()

	stat := availabilityStat(t, st, kind, id, label)
	if stat.Reported.Available {
		t.Fatalf("%s reported availability = %.3f, want unavailable", label, stat.Reported.Percent)
	}
}

func requireProbeAvailabilityPercent(t *testing.T, st *Status, kind string, id string, label string, want float64) {
	t.Helper()

	stat := availabilityStat(t, st, kind, id, label)
	requireAvailabilityMetricPercent(t, stat.Probe, label+" probe", want)
}

func requireProbeAvailabilityUnavailable(t *testing.T, st *Status, kind string, id string, label string) {
	t.Helper()

	stat := availabilityStat(t, st, kind, id, label)
	if stat.Probe.Available {
		t.Fatalf("%s probe availability = %.3f, want unavailable", label, stat.Probe.Percent)
	}
}

func requireAvailabilityMetricPercent(t *testing.T, metric availabilityMetric, name string, want float64) {
	t.Helper()

	if !metric.Available {
		t.Fatalf("%s availability is unavailable, want %.3f", name, want)
	}
	if math.Abs(metric.Percent-want) > 0.0005 {
		t.Fatalf("%s availability = %.3f, want %.3f", name, metric.Percent, want)
	}
}

func availabilityStat(t *testing.T, st *Status, kind string, id string, label string) availabilityResult {
	t.Helper()

	stats := entityAvailability(st, kind, id, fixedNow)
	for _, stat := range stats {
		if stat.Label == label {
			return stat
		}
	}

	t.Fatalf("availability label %q not found in %+v", label, stats)
	return availabilityResult{}
}

func availabilityTestOutage(id int64, beginsAt time.Time, duration time.Duration, entities []OutageEntity) *OutageReport {
	return &OutageReport{
		Id:               id,
		BeginsAt:         beginsAt,
		Duration:         duration,
		Type:             "unplanned_outage",
		State:            "resolved",
		Impact:           "full",
		EnSummary:        "Test outage",
		AffectedEntities: entities,
	}
}
