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

func TestRecordNodeStorageProbeClassifiesStorageFailures(t *testing.T) {
	tests := []struct {
		name        string
		poolStatus  bool
		poolState   string
		poolScan    string
		wantStatus  string
		wantMessage string
	}{
		{
			name:        "suspended",
			poolStatus:  true,
			poolState:   "suspended",
			poolScan:    "none",
			wantStatus:  historyProbeStateDown,
			wantMessage: "Storage not operational",
		},
		{
			name:        "faulted",
			poolStatus:  true,
			poolState:   "faulted",
			poolScan:    "none",
			wantStatus:  historyProbeStateDown,
			wantMessage: "Storage not operational",
		},
		{
			name:        "error",
			poolStatus:  true,
			poolState:   "error",
			poolScan:    "none",
			wantStatus:  historyProbeStateError,
			wantMessage: "Storage status check failed",
		},
		{
			name:        "unknown",
			poolStatus:  true,
			poolState:   "unknown",
			poolScan:    "none",
			wantStatus:  historyProbeStateError,
			wantMessage: "Unable to determine storage status",
		},
		{
			name:        "unavailable",
			poolStatus:  false,
			poolState:   "unknown",
			poolScan:    "none",
			wantStatus:  historyProbeStateError,
			wantMessage: "Unable to determine storage status",
		},
		{
			name:        "scan error",
			poolStatus:  true,
			poolState:   "online",
			poolScan:    "error",
			wantStatus:  historyProbeStateError,
			wantMessage: "Storage scan status check failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, st, _ := newTestApplication(t)
			node := &Node{
				Name:       "node1.prg",
				OsType:     "vpsadminos",
				PoolStatus: tt.poolStatus,
				PoolState:  tt.poolState,
				PoolScan:   tt.poolScan,
			}

			recordNodeStorageProbe(st, node, fixedNow)

			events := st.History.ProbeEventsFor(historyEntityNode, "node1.prg", fixedNow, historyDefaultDays)
			if len(events) != 1 {
				t.Fatalf("probe events = %+v, want one", events)
			}
			if got := events[0]; got.Method != "Storage" || got.Status != tt.wantStatus || got.Message != tt.wantMessage {
				t.Fatalf("storage event = %+v, want status=%s message=%q", got, tt.wantStatus, tt.wantMessage)
			}
		})
	}
}
