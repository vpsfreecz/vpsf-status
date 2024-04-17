package main

import (
	"log"
	"strconv"
	"time"

	"github.com/go-ping/ping"
	"github.com/prometheus/client_golang/prometheus"
)

func pingNodes(st *Status, checkInterval time.Duration) {
	for _, loc := range st.LocationList {
		for _, node := range loc.NodeList {
			go spawnPingCheck(
				node.Ping,
				st.Exporter.nodePing.With(prometheus.Labels{
					"location_id":    strconv.Itoa(loc.Id),
					"location_label": loc.Label,
					"node_id":        strconv.Itoa(node.Id),
					"node_name":      node.Name,
				}),
				checkInterval,
			)
		}
	}
}

func pingDnsResolvers(st *Status, checkInterval time.Duration) {
	for _, loc := range st.LocationList {
		for _, r := range loc.DnsResolverList {
			go spawnPingCheck(
				r.Ping,
				st.Exporter.dnsResolverPing.With(prometheus.Labels{"name": r.Name}),
				checkInterval,
			)
		}
	}
}

func pingNameServers(st *Status, checkInterval time.Duration) {
	for _, ns := range st.Services.NameServer {
		go spawnPingCheck(
			ns.Ping,
			st.Exporter.nameServerPing.With(prometheus.Labels{"name": ns.Name}),
			checkInterval,
		)
	}
}

func spawnPingCheck(pc *PingCheck, gauge prometheus.Gauge, checkInterval time.Duration) {
	for {
		pinger, err := ping.NewPinger(pc.IpAddress)
		if err != nil {
			log.Printf("Unable to create pinger for %s: %+v", pc.Name, err)
			pc.PacketLoss = 100
			gauge.Set(2)
			time.Sleep(checkInterval)
			continue
		}

		pinger.Count = 5
		pinger.Timeout = time.Duration(10 * time.Second)

		pc.LastCheck = time.Now()

		err = pinger.Run()
		if err != nil {
			log.Printf("Failed to ping resolver %s: %+v", pc.Name, err)
			pc.PacketLoss = 100
			gauge.Set(2)
			time.Sleep(time.Duration(checkInterval) * time.Second)
			continue
		}

		stats := pinger.Statistics()

		if stats.PacketsSent == stats.PacketsRecv {
			pc.PacketLoss = 0
			gauge.Set(0)
		} else {
			log.Printf("Ping stats for %s: %+v", pc.Name, stats)
			pc.PacketLoss = stats.PacketLoss

			if pc.PacketLoss < 100 {
				gauge.Set(1)
			} else {
				gauge.Set(2)
			}
		}

		time.Sleep(checkInterval)
	}
}
