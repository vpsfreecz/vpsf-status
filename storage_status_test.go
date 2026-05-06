package main

import "testing"

func TestNodeStorageStateMessages(t *testing.T) {
	tests := []struct {
		name       string
		poolStatus bool
		poolState  string
		want       string
	}{
		{
			name:       "unknown status",
			poolStatus: false,
			poolState:  "online",
			want:       "Unable to determine storage status",
		},
		{
			name:       "online",
			poolStatus: true,
			poolState:  "online",
			want:       "Storage is online",
		},
		{
			name:       "degraded",
			poolStatus: true,
			poolState:  "degraded",
			want:       "One or more disks have failed, storage continues to function",
		},
		{
			name:       "suspended",
			poolStatus: true,
			poolState:  "suspended",
			want:       "Storage not operational",
		},
		{
			name:       "faulted",
			poolStatus: true,
			poolState:  "faulted",
			want:       "Storage not operational",
		},
		{
			name:       "unknown",
			poolStatus: true,
			poolState:  "unknown",
			want:       "Unable to determine storage status",
		},
		{
			name:       "error",
			poolStatus: true,
			poolState:  "error",
			want:       "Storage status check failed",
		},
		{
			name:       "unexpected",
			poolStatus: true,
			poolState:  "mystery",
			want:       "Storage is in a unknown state",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &Node{PoolStatus: tt.poolStatus, PoolState: tt.poolState}
			if got := node.GetStorageStateMessage(); got != tt.want {
				t.Fatalf("GetStorageStateMessage = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNodeStorageScanMessages(t *testing.T) {
	tests := []struct {
		name       string
		poolStatus bool
		poolScan   string
		percent    float64
		want       string
	}{
		{
			name:       "unknown status",
			poolStatus: false,
			poolScan:   "none",
			want:       "Unable to determine storage status",
		},
		{
			name:       "none",
			poolStatus: true,
			poolScan:   "none",
			want:       "Not running",
		},
		{
			name:       "scrub",
			poolStatus: true,
			poolScan:   "scrub",
			percent:    12.5,
			want:       "Storage is being scrubbed to check data integrity, 12.5 % done",
		},
		{
			name:       "resilver",
			poolStatus: true,
			poolScan:   "resilver",
			percent:    42.5,
			want:       "Storage is being resilvered to replace disks, 42.5 % done",
		},
		{
			name:       "unknown",
			poolStatus: true,
			poolScan:   "unknown",
			want:       "Unable to determine storage scan status",
		},
		{
			name:       "error",
			poolStatus: true,
			poolScan:   "error",
			want:       "Storage scan status check failed",
		},
		{
			name:       "unexpected",
			poolStatus: true,
			poolScan:   "mystery",
			want:       "Storage scan is in a unknown state",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &Node{
				PoolStatus:      tt.poolStatus,
				PoolScan:        tt.poolScan,
				PoolScanPercent: tt.percent,
			}
			if got := node.GetStorageScanMessage(); got != tt.want {
				t.Fatalf("GetStorageScanMessage = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNodeStoragePredicates(t *testing.T) {
	unsupported := &Node{OsType: "openvz", ApiStatus: false, PoolStatus: false, PoolState: "faulted", PoolScan: "resilver"}
	if !unsupported.IsStorageOperational() || unsupported.IsStorageDegraded() {
		t.Fatalf("unsupported storage should be treated as operational: %+v", unsupported)
	}

	supported := &Node{OsType: "vpsadminos", ApiStatus: true, PoolStatus: true, PoolState: "online", PoolScan: "none"}
	if !supported.IsStorageOperational() || supported.IsStorageDegraded() {
		t.Fatalf("online storage should be operational: %+v", supported)
	}

	supported.PoolScan = "scrub"
	if !supported.IsStorageOperational() || supported.IsStorageDegraded() || !supported.IsStorageScrubIssue() {
		t.Fatalf("online storage scrub should be operational with scrub issue: %+v", supported)
	}

	supported.PoolScan = "resilver"
	if supported.IsStorageOperational() || !supported.IsStorageDegraded() || !supported.IsStorageResilverIssue() {
		t.Fatalf("online storage resilver should be degraded with resilver issue: %+v", supported)
	}

	supported.PoolState = "degraded"
	supported.PoolScan = "scrub"
	if supported.IsStorageOperational() || !supported.IsStorageDegraded() || !supported.IsStorageScrubIssue() {
		t.Fatalf("degraded storage scrub should stay degraded with scrub issue: %+v", supported)
	}

	supported.PoolState = "suspended"
	supported.PoolScan = "none"
	if supported.IsStorageOperational() || supported.IsStorageDegraded() || !supported.IsStorageHardFailure() {
		t.Fatalf("suspended storage should be a hard failure: %+v", supported)
	}

	supported.PoolState = "faulted"
	if supported.IsStorageOperational() || supported.IsStorageDegraded() || !supported.IsStorageHardFailure() {
		t.Fatalf("faulted storage should be a hard failure: %+v", supported)
	}

	supported.PoolState = "error"
	if supported.IsStorageOperational() || supported.IsStorageDegraded() || supported.IsStorageHardFailure() {
		t.Fatalf("error storage should be treated as a check failure: %+v", supported)
	}

	supported.PoolState = "online"
	supported.PoolScan = "error"
	if supported.IsStorageOperational() || supported.IsStorageDegraded() || !supported.IsStorageScanIssue() {
		t.Fatalf("scan error should be treated as a check failure: %+v", supported)
	}
}

func TestNodeStatusPredicatesPrioritizePing(t *testing.T) {
	tests := []struct {
		name            string
		apiStatus       bool
		poolStatus      bool
		poolState       string
		poolScan        string
		packetLoss      float64
		wantOperational bool
		wantDegraded    bool
	}{
		{
			name:            "all checks healthy",
			apiStatus:       true,
			poolStatus:      true,
			poolState:       "online",
			poolScan:        "none",
			packetLoss:      0,
			wantOperational: true,
		},
		{
			name:         "vpsadmin unavailable but ping responds",
			apiStatus:    false,
			poolStatus:   false,
			poolState:    "unknown",
			poolScan:     "none",
			packetLoss:   0,
			wantDegraded: true,
		},
		{
			name:         "vpsadmin unavailable with packet loss",
			apiStatus:    false,
			poolStatus:   false,
			poolState:    "unknown",
			poolScan:     "none",
			packetLoss:   40,
			wantDegraded: true,
		},
		{
			name:       "vpsadmin unavailable and ping down",
			apiStatus:  false,
			poolStatus: false,
			poolState:  "unknown",
			poolScan:   "none",
			packetLoss: 100,
		},
		{
			name:         "storage status unavailable but ping responds",
			apiStatus:    true,
			poolStatus:   false,
			poolState:    "online",
			poolScan:     "none",
			packetLoss:   0,
			wantDegraded: true,
		},
		{
			name:         "pool error is degraded when ping responds",
			apiStatus:    true,
			poolStatus:   true,
			poolState:    "error",
			poolScan:     "none",
			packetLoss:   0,
			wantDegraded: true,
		},
		{
			name:         "degraded pool is degraded when ping responds",
			apiStatus:    true,
			poolStatus:   true,
			poolState:    "degraded",
			poolScan:     "none",
			packetLoss:   0,
			wantDegraded: true,
		},
		{
			name:         "resilver is degraded when ping responds",
			apiStatus:    true,
			poolStatus:   true,
			poolState:    "online",
			poolScan:     "resilver",
			packetLoss:   0,
			wantDegraded: true,
		},
		{
			name:       "suspended pool is down even when ping responds",
			apiStatus:  true,
			poolStatus: true,
			poolState:  "suspended",
			poolScan:   "none",
			packetLoss: 0,
		},
		{
			name:       "faulted pool is down even with packet loss",
			apiStatus:  true,
			poolStatus: true,
			poolState:  "faulted",
			poolScan:   "none",
			packetLoss: 40,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &Node{
				ApiStatus:  tt.apiStatus,
				OsType:     "vpsadminos",
				PoolStatus: tt.poolStatus,
				PoolState:  tt.poolState,
				PoolScan:   tt.poolScan,
				Ping:       &PingCheck{PacketLoss: tt.packetLoss},
			}

			if got := node.IsOperational(); got != tt.wantOperational {
				t.Fatalf("IsOperational = %v, want %v", got, tt.wantOperational)
			}
			if got := node.IsDegraded(); got != tt.wantDegraded {
				t.Fatalf("IsDegraded = %v, want %v", got, tt.wantDegraded)
			}
		})
	}
}
