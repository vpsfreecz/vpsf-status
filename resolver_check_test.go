package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func TestCheckDNSResolverOnce(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus bool
		wantGauge  float64
	}{
		{name: "success", wantStatus: true, wantGauge: 0},
		{name: "lookup failure", err: errors.New("lookup failed"), wantGauge: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := &DnsResolver{Name: "resolver", IpAddress: "192.0.2.53", ResolveDomain: "example.org"}
			gauge := prometheus.NewGauge(prometheus.GaugeOpts{Name: "test_dns_lookup"})
			lookup := func(ctx context.Context, r *DnsResolver) ([]string, error) {
				if r != resolver {
					t.Fatalf("lookup resolver = %p, want %p", r, resolver)
				}
				if ctx == nil {
					t.Fatal("lookup context is nil")
				}
				if _, ok := ctx.Deadline(); !ok {
					t.Fatal("lookup context has no deadline")
				}
				if tt.err != nil {
					return nil, tt.err
				}
				return []string{"192.0.2.1"}, nil
			}

			checkDNSResolverOnce(resolver, gauge, lookup, fixedNow, time.Minute)

			if resolver.ResolveStatus != tt.wantStatus {
				t.Fatalf("ResolveStatus = %v, want %v", resolver.ResolveStatus, tt.wantStatus)
			}
			if !resolver.LastResolveCheck.Equal(fixedNow) {
				t.Fatalf("LastResolveCheck = %s, want %s", resolver.LastResolveCheck, fixedNow)
			}
			if got := gaugeValue(t, gauge); got != tt.wantGauge {
				t.Fatalf("gauge = %v, want %v", got, tt.wantGauge)
			}
		})
	}
}

func TestCheckPingOnce(t *testing.T) {
	tests := []struct {
		name       string
		result     pingResult
		err        error
		wantLoss   float64
		wantGauge  float64
		wantString string
	}{
		{
			name:       "success",
			result:     pingResult{PacketsSent: 5, PacketsRecv: 5},
			wantLoss:   0,
			wantGauge:  0,
			wantString: "responding",
		},
		{
			name:       "degraded packet loss",
			result:     pingResult{PacketsSent: 5, PacketsRecv: 3, PacketLoss: 40},
			wantLoss:   40,
			wantGauge:  1,
			wantString: "degraded",
		},
		{
			name:       "total down",
			result:     pingResult{PacketsSent: 5, PacketsRecv: 0, PacketLoss: 100},
			wantLoss:   100,
			wantGauge:  2,
			wantString: "down",
		},
		{
			name:       "runner error",
			err:        errors.New("raw socket unavailable"),
			wantLoss:   100,
			wantGauge:  2,
			wantString: "down",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pc := &PingCheck{Name: "node", IpAddress: "192.0.2.1"}
			gauge := prometheus.NewGauge(prometheus.GaugeOpts{Name: "test_ping"})
			runner := func(got *PingCheck) (pingResult, error) {
				if got != pc {
					t.Fatalf("runner ping check = %p, want %p", got, pc)
				}
				return tt.result, tt.err
			}

			checkPingOnce(pc, gauge, runner, fixedNow)

			if pc.PacketLoss != tt.wantLoss {
				t.Fatalf("PacketLoss = %v, want %v", pc.PacketLoss, tt.wantLoss)
			}
			if pc.StatusString() != tt.wantString {
				t.Fatalf("StatusString = %q, want %q", pc.StatusString(), tt.wantString)
			}
			if !pc.LastCheck.Equal(fixedNow) {
				t.Fatalf("LastCheck = %s, want %s", pc.LastCheck, fixedNow)
			}
			if got := gaugeValue(t, gauge); got != tt.wantGauge {
				t.Fatalf("gauge = %v, want %v", got, tt.wantGauge)
			}
		})
	}
}

func TestDnsResolverStatusIncludesLookupAndPing(t *testing.T) {
	resolver := &DnsResolver{
		Name:          "resolver",
		ResolveStatus: true,
		Ping:          &PingCheck{PacketLoss: 40},
	}

	if resolver.IsOperational() {
		t.Fatal("resolver with degraded ping should not be operational")
	}
	if !resolver.IsDegraded() {
		t.Fatal("resolver with successful lookup and degraded ping should be degraded")
	}

	resolver.Ping.PacketLoss = 100
	if resolver.IsOperational() || resolver.IsDegraded() {
		t.Fatal("resolver with total ping loss should be down")
	}
}
