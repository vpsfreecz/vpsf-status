package main

import (
	"testing"
	"time"
)

func TestProbeLogEventDetailViewsMarkCoveredFailures(t *testing.T) {
	_, st, _ := newTestApplication(t)
	setOperationalFixture(st)

	report := testHistoryOutage(5101, fixedNow.Add(-20*time.Minute), "Node outage", []OutageEntity{
		{Name: "Node", Id: 101, Label: "Node node1.prg"},
	})
	data := &historyData{
		reports: []*OutageReport{report},
		mapping: newHistoryEntityMapping(st),
	}

	target := ProbeTarget{
		EntityKind:  historyEntityNode,
		EntityID:    "node1.prg",
		EntityLabel: "node1.prg",
		Method:      "Ping",
	}
	events := []ProbeLogEvent{
		testProbeLogEvent(target, historyProbeStateDown, fixedNow.Add(-10*time.Minute), fixedNow.Add(-5*time.Minute)),
		testProbeLogEvent(target, historyProbeStateOperational, fixedNow.Add(-15*time.Minute), fixedNow.Add(-10*time.Minute)),
	}

	views := probeLogEventDetailViews(st, events, fixedNow, data)
	if len(views) != 2 {
		t.Fatalf("views = %+v, want 2", views)
	}
	if !views[0].HasCoverage() ||
		views[0].CoveredBy.ID != 5101 ||
		views[0].CoveredBy.Label != "Unplanned outage: Node outage" ||
		views[0].CoveredBy.URL != "https://vpsadmin.vpsfree.cz/?page=outage&action=show&id=5101" ||
		views[0].CoveredBy.Class != "danger" ||
		!views[0].GroupStart ||
		!views[0].GroupEnd {
		t.Fatalf("covered event view = %+v, want outage coverage", views[0])
	}
	if views[1].HasCoverage() {
		t.Fatalf("operational event view = %+v, want no coverage", views[1])
	}
}

func TestProbeEventResponsibleReportUsesGraceAndBestMatch(t *testing.T) {
	_, st, _ := newTestApplication(t)
	setOperationalFixture(st)

	mapping := newHistoryEntityMapping(st)
	target := ProbeTarget{
		EntityKind:  historyEntityNode,
		EntityID:    "node1.prg",
		EntityLabel: "node1.prg",
		Method:      "Ping",
	}
	entity := []OutageEntity{{Name: "Node", Id: 101, Label: "Node node1.prg"}}

	outsideGrace := testHistoryOutage(5201, fixedNow.Add(-80*time.Minute), "Outside grace", entity)
	outsideGrace.Duration = 10 * time.Minute
	event := testProbeLogEvent(target, historyProbeStateDown, fixedNow.Add(-39*time.Minute), fixedNow.Add(-30*time.Minute))
	if got := probeEventResponsibleReport(event, []*OutageReport{outsideGrace}, mapping, fixedNow); got != nil {
		t.Fatalf("outside grace report = %+v, want nil", got)
	}

	older := testHistoryOutage(5202, fixedNow.Add(-80*time.Minute), "Older outage", entity)
	older.Duration = 10 * time.Minute
	newer := testHistoryOutage(5203, fixedNow.Add(-60*time.Minute), "Newer maintenance", entity)
	newer.Duration = 10 * time.Minute
	newer.Type = "planned_outage"
	event = testProbeLogEvent(target, historyProbeStateDown, fixedNow.Add(-70*time.Minute), fixedNow.Add(-35*time.Minute))
	if got := probeEventResponsibleReport(event, []*OutageReport{older, newer}, mapping, fixedNow); got == nil || got.Id != 5203 {
		t.Fatalf("best overlap report = %+v, want newer maintenance", got)
	}

	tieA := testHistoryOutage(5204, fixedNow.Add(-50*time.Minute), "Tie A", entity)
	tieB := testHistoryOutage(5205, fixedNow.Add(-50*time.Minute), "Tie B", entity)
	event = testProbeLogEvent(target, historyProbeStateDown, fixedNow.Add(-45*time.Minute), fixedNow.Add(-44*time.Minute))
	if got := probeEventResponsibleReport(event, []*OutageReport{tieB, tieA}, mapping, fixedNow); got == nil || got.Id != 5204 {
		t.Fatalf("tie report = %+v, want lowest ID", got)
	}
}

func TestProbeLogEventDetailViewsGroupAdjacentCoverage(t *testing.T) {
	_, st, _ := newTestApplication(t)
	setOperationalFixture(st)

	report := testHistoryOutage(5301, fixedNow.Add(-20*time.Minute), "Node outage", []OutageEntity{
		{Name: "Node", Id: 101, Label: "Node node1.prg"},
	})
	data := &historyData{
		reports: []*OutageReport{report},
		mapping: newHistoryEntityMapping(st),
	}

	node1 := ProbeTarget{EntityKind: historyEntityNode, EntityID: "node1.prg", EntityLabel: "node1.prg", Method: "Ping"}
	node2 := ProbeTarget{EntityKind: historyEntityNode, EntityID: "node2.prg", EntityLabel: "node2.prg", Method: "Ping"}
	events := []ProbeLogEvent{
		testProbeLogEvent(node1, historyProbeStateDown, fixedNow.Add(-5*time.Minute), fixedNow.Add(-4*time.Minute)),
		testProbeLogEvent(node1, historyProbeStateError, fixedNow.Add(-7*time.Minute), fixedNow.Add(-6*time.Minute)),
		testProbeLogEvent(node2, historyProbeStateDown, fixedNow.Add(-9*time.Minute), fixedNow.Add(-8*time.Minute)),
		testProbeLogEvent(node1, historyProbeStateDown, fixedNow.Add(-11*time.Minute), fixedNow.Add(-10*time.Minute)),
	}

	views := probeLogEventDetailViews(st, events, fixedNow, data)
	if len(views) != 4 {
		t.Fatalf("views = %+v, want 4", views)
	}
	if !views[0].GroupStart || views[0].GroupEnd {
		t.Fatalf("first grouped event = %+v, want group start only", views[0])
	}
	if views[1].GroupStart || !views[1].GroupEnd {
		t.Fatalf("second grouped event = %+v, want group end only", views[1])
	}
	if views[2].HasCoverage() || views[2].GroupStart || views[2].GroupEnd {
		t.Fatalf("uncovered event = %+v, want no group markers", views[2])
	}
	if !views[3].GroupStart || !views[3].GroupEnd {
		t.Fatalf("separated covered event = %+v, want standalone group", views[3])
	}
}

func TestProbeEventDetailViewLocalizesCzechProbeText(t *testing.T) {
	loc, ok := newLocaleCatalog().localeForCode("cs", nil)
	if !ok {
		t.Fatal("Czech locale not found")
	}

	view := probeEventDetailViewForLocale(ProbeEvent{
		ProbeTarget: ProbeTarget{
			EntityKind:  historyEntityVpsAdmin,
			EntityID:    "console",
			EntityLabel: "Remote Console",
			Method:      "HTTP",
		},
		Status:    historyProbeStateError,
		Message:   "check failed",
		ChangedAt: fixedNow,
	}, loc)

	if view.Entity != "Vzdálená konzole" ||
		view.Method != "HTTP" ||
		view.Message != "kontrola selhala" {
		t.Fatalf("localized probe event = %+v", view)
	}
}

func TestProbeEventDetailViewLocalizesCzechStorageMessages(t *testing.T) {
	loc, ok := newLocaleCatalog().localeForCode("cs", nil)
	if !ok {
		t.Fatal("Czech locale not found")
	}

	tests := []struct {
		message string
		want    string
	}{
		{"Storage not operational", "Úložiště není funkční"},
		{"Unable to determine storage status", "Nepodařilo se zjistit status úložiště"},
		{"Storage is being scrubbed to check data integrity, 12.5 % done", "Na úložišti běží scrub pro kontrolu integrity dat, hotovo 12.5 %"},
		{"Storage is being resilvered to replace disks, 42.5 % done", "Na úložišti běží resilver kvůli náhradě disků, hotovo 42.5 %"},
	}

	for _, tt := range tests {
		t.Run(tt.message, func(t *testing.T) {
			view := probeEventDetailViewForLocale(ProbeEvent{
				ProbeTarget: ProbeTarget{
					EntityKind:  historyEntityNode,
					EntityID:    "node1.prg",
					EntityLabel: "node1.prg",
					Method:      "Storage",
				},
				Status:    historyProbeStateError,
				Message:   tt.message,
				ChangedAt: fixedNow,
			}, loc)

			if view.Message != tt.want {
				t.Fatalf("message = %q, want %q", view.Message, tt.want)
			}
		})
	}
}

func testProbeLogEvent(target ProbeTarget, status string, startsAt time.Time, endsAt time.Time) ProbeLogEvent {
	return ProbeLogEvent{
		ProbeEvent: ProbeEvent{
			ProbeTarget: target,
			Status:      status,
			Message:     status,
			ChangedAt:   startsAt,
		},
		EndsAt: endsAt,
	}
}
