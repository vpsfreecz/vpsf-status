package main

import (
	"strings"
	"testing"
	"time"
)

func TestRecordNodeStorageProbeTreatsOnlineScrubAsOperational(t *testing.T) {
	_, st, _ := newTestApplication(t)

	node := &Node{
		Name:            "node1.prg",
		OsType:          "vpsadminos",
		ApiStatus:       true,
		PoolStatus:      true,
		PoolState:       "online",
		PoolScan:        "scrub",
		PoolScanPercent: 15,
	}
	recordNodeStorageProbe(st, node, fixedNow)

	events := st.History.ProbeEventsFor(historyEntityNode, "node1.prg", fixedNow, historyDefaultDays)
	if len(events) != 1 {
		t.Fatalf("probe events = %+v, want one", events)
	}
	if got := events[0]; got.Method != "Storage" || got.Status != historyProbeStateOperational || got.Message != "online" {
		t.Fatalf("online scrub event = %+v, want operational storage event", got)
	}

	node.PoolState = "degraded"
	recordNodeStorageProbe(st, node, fixedNow.Add(time.Minute))

	events = st.History.ProbeEventsFor(historyEntityNode, "node1.prg", fixedNow.Add(time.Minute), historyDefaultDays)
	if len(events) != 2 {
		t.Fatalf("probe events = %+v, want two", events)
	}
	if got := events[0]; got.Method != "Storage" ||
		got.Status != historyProbeStateDegraded ||
		!strings.Contains(got.Message, "Storage is being scrubbed") {
		t.Fatalf("degraded scrub event = %+v, want degraded storage scrub event", got)
	}
}
