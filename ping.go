package main

import (
	"log"
	"strconv"
	"time"

	"github.com/go-ping/ping"
	"github.com/prometheus/client_golang/prometheus"
)

type pingResult struct {
	PacketsSent int
	PacketsRecv int
	PacketLoss  float64
}

type pingRunner func(*PingCheck) (pingResult, error)

func pingNodes(st *Status, checkInterval time.Duration) {
	for _, loc := range st.LocationList {
		for _, node := range loc.NodeList {
			go spawnPingCheck(
				st,
				node.Ping,
				ProbeTarget{EntityKind: historyEntityNode, EntityID: node.Name, EntityLabel: node.Name, Method: "Ping"},
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
				st,
				r.Ping,
				ProbeTarget{EntityKind: historyEntityDnsResolver, EntityID: r.Name, EntityLabel: r.Name, Method: "Ping"},
				st.Exporter.dnsResolverPing.With(prometheus.Labels{"name": r.Name}),
				checkInterval,
			)
		}
	}
}

func pingNameServers(st *Status, checkInterval time.Duration) {
	for _, ns := range st.Services.NameServer {
		go spawnPingCheck(
			st,
			ns.Ping,
			ProbeTarget{EntityKind: historyEntityNameServer, EntityID: ns.Name, EntityLabel: ns.Name, Method: "Ping"},
			st.Exporter.nameServerPing.With(prometheus.Labels{"name": ns.Name}),
			checkInterval,
		)
	}
}

func spawnPingCheck(st *Status, pc *PingCheck, target ProbeTarget, gauge prometheus.Gauge, checkInterval time.Duration) {
	for {
		now := time.Now()
		checkPingOnce(pc, gauge, runPing, now)
		status, message := pingProbeStatus(pc)
		recordProbeStatus(st, target, status, message, now)
		st.requestIndexRenderIfConfigured()
		time.Sleep(checkInterval)
	}
}

func checkPingOnce(pc *PingCheck, gauge prometheus.Gauge, runner pingRunner, now time.Time) {
	pc.LastCheck = now

	stats, err := runner(pc)
	if err != nil {
		log.Printf("Failed to ping %s: %+v", pc.Name, err)
		pc.PacketLoss = 100
		gauge.Set(2)
		return
	}

	if stats.PacketsSent == stats.PacketsRecv {
		pc.PacketLoss = 0
		gauge.Set(0)
		return
	}

	log.Printf("Ping stats for %s: %+v", pc.Name, stats)
	pc.PacketLoss = stats.PacketLoss

	if pc.PacketLoss < 100 {
		gauge.Set(1)
	} else {
		gauge.Set(2)
	}
}

func runPing(pc *PingCheck) (pingResult, error) {
	pinger, err := ping.NewPinger(pc.IpAddress)
	if err != nil {
		return pingResult{}, err
	}

	pinger.Count = 5
	pinger.Timeout = 10 * time.Second

	if err := pinger.Run(); err != nil {
		return pingResult{}, err
	}

	stats := pinger.Statistics()
	return pingResult{
		PacketsSent: stats.PacketsSent,
		PacketsRecv: stats.PacketsRecv,
		PacketLoss:  stats.PacketLoss,
	}, nil
}
