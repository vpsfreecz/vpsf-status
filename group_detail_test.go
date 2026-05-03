package main

import (
	"testing"
	"time"
)

func TestGroupAvailabilityAveragesOnlyAvailableMemberData(t *testing.T) {
	_, st, _ := newTestApplication(t)
	setOperationalFixture(st)

	loc := st.LocationMap["Praha"]
	windowStart := fixedNow.AddDate(0, 0, -30)
	reports := []*OutageReport{
		availabilityTestOutage(7201, windowStart.Add(24*time.Hour), 48*time.Hour, []OutageEntity{
			{Name: "Node", Id: 101, Label: "Node node1.prg"},
		}),
		availabilityTestOutage(7202, windowStart.Add(24*time.Hour), 48*time.Hour, []OutageEntity{
			{Name: "DNS Resolver", Label: "resolver-prg"},
		}),
	}
	if err := st.History.ReplaceOutages(reports, fixedNow); err != nil {
		t.Fatalf("replace outages: %v", err)
	}

	if err := st.History.RecordProbeStatus(ProbeTarget{
		EntityKind:  historyEntityNode,
		EntityID:    "node1.prg",
		EntityLabel: "node1.prg",
		Method:      "Ping",
	}, historyProbeStateOperational, "responding", windowStart.Add(-time.Hour)); err != nil {
		t.Fatalf("record probe status: %v", err)
	}

	stats := groupAvailability(st, locationGroupTargets(loc), locationReportedTargets(loc), fixedNow)
	stat := availabilityResultByLabel(t, stats, "30 days")
	requireAvailabilityMetricPercent(t, stat.Reported, "location reported", 96.667)
	requireAvailabilityMetricPercent(t, stat.Probe, "location probe", 100)
}

func availabilityResultByLabel(t *testing.T, stats []availabilityResult, label string) availabilityResult {
	t.Helper()

	for _, stat := range stats {
		if stat.Label == label {
			return stat
		}
	}

	t.Fatalf("availability label %q not found in %+v", label, stats)
	return availabilityResult{}
}
