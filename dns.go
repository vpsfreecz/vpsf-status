package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type dnsLookupFunc func(context.Context, *DnsResolver) ([]string, error)

func checkDnsResolvers(st *Status, checkInterval time.Duration) {
	for _, loc := range st.LocationList {
		for _, r := range loc.DnsResolverList {
			go spawnDnsResolverCheck(
				st,
				r,
				ProbeTarget{EntityKind: historyEntityDnsResolver, EntityID: r.Name, EntityLabel: r.Name, Method: "Lookup"},
				st.Exporter.dnsResolverLookup.With(prometheus.Labels{"name": r.Name}),
				checkInterval,
			)
		}
	}
}

func checkNameServers(st *Status, checkInterval time.Duration) {
	for _, ns := range st.Services.NameServer {
		go spawnDnsResolverCheck(
			st,
			ns,
			ProbeTarget{EntityKind: historyEntityNameServer, EntityID: ns.Name, EntityLabel: ns.Name, Method: "Lookup"},
			st.Exporter.nameServerLookup.With(prometheus.Labels{"name": ns.Name}),
			checkInterval,
		)
	}
}

func spawnDnsResolverCheck(st *Status, r *DnsResolver, target ProbeTarget, gauge prometheus.Gauge, checkInterval time.Duration) {
	for {
		now := time.Now()
		checkDNSResolverOnce(r, gauge, lookupThroughDNSResolver, now)
		status := historyProbeStateError
		message := "lookup failed"
		if r.ResolveStatus {
			status = historyProbeStateOperational
			message = "lookup succeeded"
		}
		recordProbeStatus(st, target, status, message, now)
		time.Sleep(checkInterval)
	}
}

func checkDNSResolverOnce(r *DnsResolver, gauge prometheus.Gauge, lookup dnsLookupFunc, now time.Time) {
	r.LastResolveCheck = now

	_, err := lookup(context.Background(), r)
	if err != nil {
		log.Printf("DNS lookup failed on %s", r.Name)
		r.ResolveStatus = false
		gauge.Set(1)
		return
	}

	r.ResolveStatus = true
	gauge.Set(0)
}

func lookupThroughDNSResolver(ctx context.Context, r *DnsResolver) ([]string, error) {
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: 10 * time.Second,
			}
			return d.DialContext(ctx, network, fmt.Sprintf("%s:53", r.IpAddress))
		},
	}

	return resolver.LookupHost(ctx, r.ResolveDomain)
}
