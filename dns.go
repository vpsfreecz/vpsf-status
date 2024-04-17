package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func checkDnsResolvers(st *Status, checkInterval time.Duration) {
	for _, loc := range st.LocationList {
		for _, r := range loc.DnsResolverList {
			go spawnDnsResolverCheck(
				r,
				st.Exporter.dnsResolverLookup.With(prometheus.Labels{"name": r.Name}),
				checkInterval,
			)
		}
	}
}

func checkNameServers(st *Status, checkInterval time.Duration) {
	for _, ns := range st.Services.NameServer {
		go spawnDnsResolverCheck(
			ns,
			st.Exporter.nameServerLookup.With(prometheus.Labels{"name": ns.Name}),
			checkInterval,
		)
	}
}

func spawnDnsResolverCheck(r *DnsResolver, gauge prometheus.Gauge, checkInterval time.Duration) {
	for {
		net_r := &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{
					Timeout: time.Millisecond * time.Duration(10000),
				}
				return d.DialContext(ctx, network, fmt.Sprintf("%s:53", r.IpAddress))
			},
		}

		r.LastResolveCheck = time.Now()

		_, err := net_r.LookupHost(context.Background(), r.ResolveDomain)
		if err != nil {
			log.Printf("DNS lookup failed on %s", r.Name)
			r.ResolveStatus = false
			gauge.Set(1)
			time.Sleep(checkInterval)
			continue
		}

		r.ResolveStatus = true
		gauge.Set(0)
		time.Sleep(checkInterval)
	}
}
