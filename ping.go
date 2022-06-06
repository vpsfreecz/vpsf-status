package main

import (
	"log"
	"time"

	"github.com/go-ping/ping"
)

func pingNodes(st *Status, checkInterval time.Duration) {
	for _, loc := range st.LocationList {
		for _, node := range loc.NodeList {
			go spawnPingCheck(node.Ping, checkInterval)
		}
	}
}

func pingDnsResolvers(st *Status, checkInterval time.Duration) {
	for _, loc := range st.LocationList {
		for _, r := range loc.DnsResolverList {
			go spawnPingCheck(r.Ping, checkInterval)
		}
	}
}

func spawnPingCheck(pc *PingCheck, checkInterval time.Duration) {
	for {
		pinger, err := ping.NewPinger(pc.IpAddress)
		if err != nil {
			log.Printf("Unable to create pinger for %s: %+v", pc.Name, err)
			pc.PacketLoss = 100
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
			time.Sleep(time.Duration(checkInterval) * time.Second)
			continue
		}

		stats := pinger.Statistics()

		if stats.PacketsSent == stats.PacketsRecv {
			pc.PacketLoss = 0
		} else {
			log.Printf("Ping stats for %s: %+v", pc.Name, stats)
			pc.PacketLoss = stats.PacketLoss
		}

		time.Sleep(checkInterval)
	}
}
