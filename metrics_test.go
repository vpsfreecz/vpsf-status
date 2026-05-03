package main

import (
	"net/http"
	"testing"
)

func TestRoutesServePrometheusMetricsContract(t *testing.T) {
	app, st, cfg := newTestApplication(t)
	setOperationalFixture(st)
	writeNotice(t, cfg, "<p>Notice</p>")
	st.Exporter.notice.Set(1)

	setVpsAdminService(st, "webui", st.VpsAdmin.Webui, false, true, http.StatusServiceUnavailable)
	setVpsAdminService(st, "console", st.VpsAdmin.Console, false, false, http.StatusInternalServerError)

	node := st.GlobalNodeMap["node2.prg"]
	setNodeState(st, st.LocationMap["Praha"], node, true, false, "vpsadminos", true, "degraded", "resilver", 42.5, 40)

	dns := st.LocationMap["Praha"].DnsResolverList[0]
	setResolverState(st.Exporter.dnsResolverPing, st.Exporter.dnsResolverLookup, dns, 40, false)

	setWebServiceState(st, st.Services.Web[1], false, false, http.StatusInternalServerError)
	setResolverState(st.Exporter.nameServerPing, st.Exporter.nameServerLookup, st.Services.NameServer[0], 100, true)

	families := scrapeMetrics(t, app)
	requireMetricFamilies(
		t,
		families,
		"vpsfstatus_dns_resolver_lookup",
		"vpsfstatus_dns_resolver_ping_status",
		"vpsfstatus_nameserver_lookup",
		"vpsfstatus_nameserver_ping_status",
		"vpsfstatus_index_last_render_timestamp_seconds",
		"vpsfstatus_node_ping_status",
		"vpsfstatus_node_pool_scan",
		"vpsfstatus_node_pool_scan_percent",
		"vpsfstatus_node_pool_state",
		"vpsfstatus_node_pool_status",
		"vpsfstatus_node_vpsadmin_status",
		"vpsfstatus_notice",
		"vpsfstatus_up",
		"vpsfstatus_vpsadmin_status",
		"vpsfstatus_web_service_status",
	)

	up := requireGaugeFamily(t, families, "vpsfstatus_up", "1 = operational, 0 = initializing")
	requireMetricCount(t, up, 1)
	requireUnlabeledMetricValue(t, up, 1)
	notice := requireGaugeFamily(t, families, "vpsfstatus_notice", "0 = no issue reported, 1 = there is a notice")
	requireMetricCount(t, notice, 1)
	requireUnlabeledMetricValue(t, notice, 1)
	indexLastRender := requireGaugeFamily(t, families, "vpsfstatus_index_last_render_timestamp_seconds", "Unix timestamp of the last successful index page render.")
	requireMetricCount(t, indexLastRender, 1)
	requireUnlabeledMetricValue(t, indexLastRender, 0)

	vpsAdmin := requireGaugeFamily(t, families, "vpsfstatus_vpsadmin_status", "0 = operational, 1 = under maintenance, 2 = down")
	requireMetricCount(t, vpsAdmin, 3)
	requireMetricLabelSets(t, vpsAdmin, map[string]string{"service": "api"}, map[string]string{"service": "webui"}, map[string]string{"service": "console"})
	requireMetricValue(t, vpsAdmin, map[string]string{"service": "api"}, 0)
	requireMetricValue(t, vpsAdmin, map[string]string{"service": "webui"}, 1)
	requireMetricValue(t, vpsAdmin, map[string]string{"service": "console"}, 2)

	nodeLabels := map[string]string{
		"location_id":    "3",
		"location_label": "Praha",
		"node_id":        "102",
		"node_name":      "node2.prg",
	}
	allNodeLabels := []map[string]string{
		{"location_id": "3", "location_label": "Praha", "node_id": "101", "node_name": "node1.prg"},
		nodeLabels,
		{"location_id": "4", "location_label": "Brno", "node_id": "201", "node_name": "node1.brq"},
	}
	nodeFamilies := map[string]string{
		"vpsfstatus_node_vpsadmin_status":   "0 = vpsAdmin is up, 1 = under maintenance, 2 = down",
		"vpsfstatus_node_ping_status":       "0 = responding, 1 = degraded, 2 = not responding",
		"vpsfstatus_node_pool_state":        "0 = unknown, 1 = online, 2 = degraded, 3 = suspended, 4 = faulted, 5 = error",
		"vpsfstatus_node_pool_scan":         "0 = none, 1 = scrub, 2 = resilver",
		"vpsfstatus_node_pool_status":       "0 = pool status is known, 1 = under maintenance, 2 = pool status is not known",
		"vpsfstatus_node_pool_scan_percent": "Pool scan progress in percent",
	}
	for name, help := range nodeFamilies {
		family := requireGaugeFamily(t, families, name, help)
		requireMetricCount(t, family, 3)
		requireMetricLabelSets(t, family, allNodeLabels...)
	}
	requireMetricValue(t, families["vpsfstatus_node_vpsadmin_status"], nodeLabels, 0)
	requireMetricValue(t, families["vpsfstatus_node_ping_status"], nodeLabels, 1)
	requireMetricValue(t, families["vpsfstatus_node_pool_state"], nodeLabels, 2)
	requireMetricValue(t, families["vpsfstatus_node_pool_scan"], nodeLabels, 2)
	requireMetricValue(t, families["vpsfstatus_node_pool_scan_percent"], nodeLabels, 42.5)
	requireMetricValue(t, families["vpsfstatus_node_pool_status"], nodeLabels, 0)

	operationalNodeLabels := map[string]string{
		"location_id":    "3",
		"location_label": "Praha",
		"node_id":        "101",
		"node_name":      "node1.prg",
	}
	requireMetricValue(t, families["vpsfstatus_node_vpsadmin_status"], operationalNodeLabels, 0)
	requireMetricValue(t, families["vpsfstatus_node_ping_status"], operationalNodeLabels, 0)
	requireMetricValue(t, families["vpsfstatus_node_pool_state"], operationalNodeLabels, 1)
	requireMetricValue(t, families["vpsfstatus_node_pool_scan"], operationalNodeLabels, 0)
	requireMetricValue(t, families["vpsfstatus_node_pool_scan_percent"], operationalNodeLabels, 0)
	requireMetricValue(t, families["vpsfstatus_node_pool_status"], operationalNodeLabels, 0)

	dnsResolverPing := requireGaugeFamily(t, families, "vpsfstatus_dns_resolver_ping_status", "0 = responding, 1 = degraded, 2 = not responding")
	requireMetricCount(t, dnsResolverPing, 1)
	requireMetricLabelSets(t, dnsResolverPing, map[string]string{"name": "resolver-prg"})
	requireMetricValue(t, dnsResolverPing, map[string]string{"name": "resolver-prg"}, 1)
	dnsResolverLookup := requireGaugeFamily(t, families, "vpsfstatus_dns_resolver_lookup", "0 = operational, 1 = not operational")
	requireMetricCount(t, dnsResolverLookup, 1)
	requireMetricLabelSets(t, dnsResolverLookup, map[string]string{"name": "resolver-prg"})
	requireMetricValue(t, dnsResolverLookup, map[string]string{"name": "resolver-prg"}, 1)

	webServices := requireGaugeFamily(t, families, "vpsfstatus_web_service_status", "0 = service is up, 1 = under maintenance, 2 = down")
	requireMetricCount(t, webServices, 2)
	requireMetricLabelSets(t, webServices, map[string]string{"service": "vpsfree.cz"}, map[string]string{"service": "kb.vpsfree.cz"})
	requireMetricValue(t, webServices, map[string]string{"service": "vpsfree.cz"}, 0)
	requireMetricValue(t, webServices, map[string]string{"service": "kb.vpsfree.cz"}, 2)

	nameServerPing := requireGaugeFamily(t, families, "vpsfstatus_nameserver_ping_status", "0 = responding, 1 = degraded, 2 = not responding")
	requireMetricCount(t, nameServerPing, 1)
	requireMetricLabelSets(t, nameServerPing, map[string]string{"name": "ns1.vpsfree.cz"})
	requireMetricValue(t, nameServerPing, map[string]string{"name": "ns1.vpsfree.cz"}, 2)
	nameServerLookup := requireGaugeFamily(t, families, "vpsfstatus_nameserver_lookup", "0 = operational, 1 = not operational")
	requireMetricCount(t, nameServerLookup, 1)
	requireMetricLabelSets(t, nameServerLookup, map[string]string{"name": "ns1.vpsfree.cz"})
	requireMetricValue(t, nameServerLookup, map[string]string{"name": "ns1.vpsfree.cz"}, 0)
}

func TestRoutesServePrometheusInitialGaugeDefaults(t *testing.T) {
	app, _, _ := newTestApplication(t)

	families := scrapeMetrics(t, app)

	up := requireGaugeFamily(t, families, "vpsfstatus_up", "1 = operational, 0 = initializing")
	requireUnlabeledMetricValue(t, up, 0)
	notice := requireGaugeFamily(t, families, "vpsfstatus_notice", "0 = no issue reported, 1 = there is a notice")
	requireUnlabeledMetricValue(t, notice, 0)
	indexLastRender := requireGaugeFamily(t, families, "vpsfstatus_index_last_render_timestamp_seconds", "Unix timestamp of the last successful index page render.")
	requireUnlabeledMetricValue(t, indexLastRender, 0)
}

func TestRoutesServePrometheusNodeMetricValuesForDownNode(t *testing.T) {
	app, st, _ := newTestApplication(t)
	setOperationalFixture(st)

	node := st.GlobalNodeMap["node2.prg"]
	setNodeState(st, st.LocationMap["Praha"], node, false, false, "vpsadminos", false, "unknown", "none", 0, 100)

	families := scrapeMetrics(t, app)
	labels := map[string]string{
		"location_id":    "3",
		"location_label": "Praha",
		"node_id":        "102",
		"node_name":      "node2.prg",
	}
	requireMetricValue(t, families["vpsfstatus_node_vpsadmin_status"], labels, 2)
	requireMetricValue(t, families["vpsfstatus_node_ping_status"], labels, 2)
	requireMetricValue(t, families["vpsfstatus_node_pool_state"], labels, 0)
	requireMetricValue(t, families["vpsfstatus_node_pool_scan"], labels, 0)
	requireMetricValue(t, families["vpsfstatus_node_pool_scan_percent"], labels, 0)
	requireMetricValue(t, families["vpsfstatus_node_pool_status"], labels, 2)
}
