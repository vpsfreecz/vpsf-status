package main

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Exporter struct {
	registry            *prometheus.Registry
	up                  prometheus.Gauge
	notice              prometheus.Gauge
	vpsAdminStatus      *prometheus.GaugeVec
	nodeVpsAdminStatus  *prometheus.GaugeVec
	nodePing            *prometheus.GaugeVec
	nodePoolState       *prometheus.GaugeVec
	nodePoolStatus      *prometheus.GaugeVec
	nodePoolScan        *prometheus.GaugeVec
	nodePoolScanPercent *prometheus.GaugeVec
	dnsResolverPing     *prometheus.GaugeVec
	dnsResolverLookup   *prometheus.GaugeVec
	webServiceStatus    *prometheus.GaugeVec
	nameServerPing      *prometheus.GaugeVec
	nameServerLookup    *prometheus.GaugeVec
}

func newExporter() *Exporter {
	exporter := Exporter{
		registry: prometheus.NewRegistry(),
		up: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "vpsfstatus_up",
				Help: "1 = operational, 0 = initializing",
			},
		),
		notice: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "vpsfstatus_notice",
				Help: "0 = no issue reported, 1 = there is a notice",
			},
		),
		vpsAdminStatus: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "vpsfstatus_vpsadmin_status",
				Help: "0 = operational, 1 = under maintenance, 2 = down",
			},
			[]string{"service"},
		),
		nodeVpsAdminStatus: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "vpsfstatus_node_vpsadmin_status",
				Help: "0 = vpsAdmin is up, 1 = under maintenance, 2 = down",
			},
			[]string{"location_id", "location_label", "node_id", "node_name"},
		),
		nodePing: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "vpsfstatus_node_ping_status",
				Help: "0 = responding, 1 = degraded, 2 = not responding",
			},
			[]string{"location_id", "location_label", "node_id", "node_name"},
		),
		nodePoolState: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "vpsfstatus_node_pool_state",
				Help: "0 = unknown, 1 = online, 2 = degraded, 3 = suspended, 4 = faulted, 5 = error",
			},
			[]string{"location_id", "location_label", "node_id", "node_name"},
		),
		nodePoolScan: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "vpsfstatus_node_pool_scan",
				Help: "0 = none, 1 = scrub, 2 = resilver",
			},
			[]string{"location_id", "location_label", "node_id", "node_name"},
		),
		nodePoolScanPercent: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "vpsfstatus_node_pool_scan_percent",
				Help: "Pool scan progress in percent",
			},
			[]string{"location_id", "location_label", "node_id", "node_name"},
		),
		nodePoolStatus: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "vpsfstatus_node_pool_status",
				Help: "0 = pool status is known, 1 = under maintenance, 2 = pool status is not known",
			},
			[]string{"location_id", "location_label", "node_id", "node_name"},
		),
		dnsResolverPing: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "vpsfstatus_dns_resolver_ping_status",
				Help: "0 = responding, 1 = degraded, 2 = not responding",
			},
			[]string{"name"},
		),
		dnsResolverLookup: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "vpsfstatus_dns_resolver_lookup",
				Help: "0 = operational, 1 = not operational",
			},
			[]string{"name"},
		),
		webServiceStatus: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "vpsfstatus_web_service_status",
				Help: "0 = service is up, 1 = under maintenance, 2 = down",
			},
			[]string{"service"},
		),
		nameServerPing: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "vpsfstatus_nameserver_ping_status",
				Help: "0 = responding, 1 = degraded, 2 = not responding",
			},
			[]string{"name"},
		),
		nameServerLookup: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "vpsfstatus_nameserver_lookup",
				Help: "0 = operational, 1 = not operational",
			},
			[]string{"name"},
		),
	}

	exporter.registry.MustRegister(
		exporter.up,
		exporter.notice,
		exporter.vpsAdminStatus,
		exporter.nodeVpsAdminStatus,
		exporter.nodePing,
		exporter.nodePoolState,
		exporter.nodePoolScan,
		exporter.nodePoolScanPercent,
		exporter.nodePoolStatus,
		exporter.dnsResolverPing,
		exporter.dnsResolverLookup,
		exporter.webServiceStatus,
		exporter.nameServerPing,
		exporter.nameServerLookup,
	)

	exporter.up.Set(0)

	return &exporter
}

func (ex *Exporter) httpHandler() http.Handler {
	return promhttp.HandlerFor(
		ex.registry,
		promhttp.HandlerOpts{
			EnableOpenMetrics: true,
		},
	)
}
