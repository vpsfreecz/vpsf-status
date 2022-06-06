package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"
)

func checkDnsResolvers(st *Status, checkInterval time.Duration) {
	for _, loc := range st.LocationList {
		for _, r := range loc.DnsResolverList {
			go spawnDnsResolverCheck(r, checkInterval)
		}
	}
}

func checkNameServers(st *Status, checkInterval time.Duration) {
	for _, ns := range st.Services.NameServer {
		go spawnDnsResolverCheck(ns, checkInterval)
	}
}

func spawnDnsResolverCheck(r *DnsResolver, checkInterval time.Duration) {
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
			time.Sleep(checkInterval)
			continue
		}

		r.ResolveStatus = true
		time.Sleep(checkInterval)
	}
}
