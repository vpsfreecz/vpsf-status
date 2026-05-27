package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/vpsfreecz/vpsf-status/config"
)

var (
	fixedNow        = time.Date(2026, 5, 2, 10, 30, 0, 0, time.UTC)
	fixedNoticeTime = time.Date(2026, 5, 2, 9, 15, 0, 0, time.UTC)
)

func newTestApplication(t testing.TB) (*application, *Status, *config.Config) {
	t.Helper()

	cfg := &config.Config{
		ListenAddress: ":0",
		DataDir:       ".",
		StateDir:      filepath.Join(t.TempDir(), "history"),
		NoticeFile:    filepath.Join(t.TempDir(), "notice.html"),
		CheckInterval: 30,
		VpsAdmin: config.VpsAdmin{
			ApiUrl:     "https://api.vpsfree.cz",
			WebuiUrl:   "https://vpsadmin.vpsfree.cz",
			ConsoleUrl: "https://console.vpsfree.cz/vzconsole.js",
		},
		Locations: []config.Location{
			{
				Id:    3,
				Label: "Praha",
				Nodes: []config.Node{
					{Id: 101, Name: "node1.prg", IpAddress: "172.16.0.10"},
					{Id: 102, Name: "node2.prg", IpAddress: "172.16.0.11"},
				},
				DnsResolvers: []config.DnsResolver{
					{Name: "resolver-prg", IpAddress: "172.16.0.53"},
				},
			},
			{
				Id:    4,
				Label: "Brno",
				Nodes: []config.Node{
					{Id: 201, Name: "node1.brq", IpAddress: "172.19.0.10"},
				},
			},
		},
		WebServices: []config.WebService{
			{
				Label:       "vpsfree.cz",
				Description: "Website in Czech",
				Url:         "https://vpsfree.cz",
				CheckUrl:    "https://check.vpsfree.cz/",
			},
			{
				Label:       "kb.vpsfree.cz",
				Description: "Knowledge Base in Czech",
				Url:         "https://kb.vpsfree.cz",
			},
		},
		NameServers: []config.NameServer{
			{Name: "ns1.vpsfree.cz", Domain: "vpsfree.cz"},
		},
	}

	st := openConfig(cfg)
	history, err := openHistoryStore(cfg.StateDir)
	if err != nil {
		t.Fatalf("open history store: %v", err)
	}
	st.History = history
	app := &application{
		config: cfg,
		status: st,
		now: func() time.Time {
			return fixedNow
		},
	}

	if err := app.parseTemplates(); err != nil {
		t.Fatalf("parse templates: %v", err)
	}

	return app, st, cfg
}

func setOperationalFixture(st *Status) {
	st.Initialized = true
	st.Exporter.up.Set(1)
	st.Exporter.notice.Set(0)

	st.OutageReports = &OutageReports{
		Status:     true,
		ActiveList: make([]*OutageReport, 0),
		RecentList: make([]*OutageReport, 0),
	}

	setVpsAdminService(st, "api", st.VpsAdmin.Api, true, false, http.StatusOK)
	setVpsAdminService(st, "webui", st.VpsAdmin.Webui, true, false, http.StatusOK)
	setVpsAdminService(st, "console", st.VpsAdmin.Console, true, false, http.StatusOK)

	for _, loc := range st.LocationList {
		for _, node := range loc.NodeList {
			setNodeState(st, loc, node, true, false, "vpsadminos", true, "online", "none", 0, 0)
		}

		for _, dns := range loc.DnsResolverList {
			setResolverState(st.Exporter.dnsResolverPing, st.Exporter.dnsResolverLookup, dns, 0, true)
		}
	}

	for _, ws := range st.Services.Web {
		setWebServiceState(st, ws, true, false, http.StatusOK)
	}

	for _, ns := range st.Services.NameServer {
		setResolverState(st.Exporter.nameServerPing, st.Exporter.nameServerLookup, ns, 0, true)
	}
}

func setVpsAdminService(st *Status, service string, ws *WebService, status bool, maintenance bool, statusCode int) {
	ws.Status = status
	ws.Maintenance = maintenance
	ws.StatusCode = statusCode
	ws.LastCheck = fixedNow

	st.Exporter.vpsAdminStatus.WithLabelValues(service).Set(webServiceMetricValue(ws))
}

func setWebServiceState(st *Status, ws *WebService, status bool, maintenance bool, statusCode int) {
	ws.Status = status
	ws.Maintenance = maintenance
	ws.StatusCode = statusCode
	ws.LastCheck = fixedNow

	st.Exporter.webServiceStatus.WithLabelValues(ws.Label).Set(webServiceMetricValue(ws))
}

func webServiceMetricValue(ws *WebService) float64 {
	if ws.Status {
		return 0
	}

	if ws.Maintenance {
		return 1
	}

	return 2
}

func setNodeState(
	st *Status,
	loc *Location,
	node *Node,
	apiStatus bool,
	apiMaintenance bool,
	osType string,
	poolStatus bool,
	poolState string,
	poolScan string,
	poolScanPercent float64,
	packetLoss float64,
) {
	node.ApiStatus = apiStatus
	node.ApiMaintenance = apiMaintenance
	node.LastApiCheck = fixedNow
	node.LocationId = loc.Id
	node.OsType = osType
	node.PoolStatus = poolStatus
	node.PoolState = poolState
	node.PoolScan = poolScan
	node.PoolScanPercent = poolScanPercent
	node.Ping.PacketLoss = packetLoss
	node.Ping.LastCheck = fixedNow

	labels := testNodePrometheusLabels(loc, node)
	if apiMaintenance {
		st.Exporter.nodeVpsAdminStatus.With(labels).Set(1)
		st.Exporter.nodePoolStatus.With(labels).Set(1)
	} else {
		if apiStatus {
			st.Exporter.nodeVpsAdminStatus.With(labels).Set(0)
		} else {
			st.Exporter.nodeVpsAdminStatus.With(labels).Set(2)
		}

		if poolStatus {
			st.Exporter.nodePoolStatus.With(labels).Set(0)
		} else {
			st.Exporter.nodePoolStatus.With(labels).Set(2)
		}
	}

	st.Exporter.nodePing.With(labels).Set(pingMetricValue(packetLoss))
	st.Exporter.nodePoolState.With(labels).Set(indexMetricValue(poolStates, poolState))
	st.Exporter.nodePoolScan.With(labels).Set(indexMetricValue(poolScans, poolScan))
	st.Exporter.nodePoolScanPercent.With(labels).Set(poolScanPercent)
}

func setResolverState(pingGauge *prometheus.GaugeVec, lookupGauge *prometheus.GaugeVec, resolver *DnsResolver, packetLoss float64, lookup bool) {
	resolver.Ping.PacketLoss = packetLoss
	resolver.Ping.LastCheck = fixedNow
	resolver.ResolveStatus = lookup
	resolver.LastResolveCheck = fixedNow

	pingGauge.WithLabelValues(resolver.Name).Set(pingMetricValue(packetLoss))
	if lookup {
		lookupGauge.WithLabelValues(resolver.Name).Set(0)
	} else {
		lookupGauge.WithLabelValues(resolver.Name).Set(1)
	}
}

func testNodePrometheusLabels(loc *Location, node *Node) prometheus.Labels {
	return prometheus.Labels{
		"location_id":    fmt.Sprintf("%d", loc.Id),
		"location_label": loc.Label,
		"node_id":        fmt.Sprintf("%d", node.Id),
		"node_name":      node.Name,
	}
}

func pingMetricValue(packetLoss float64) float64 {
	if packetLoss <= 20 {
		return 0
	}

	if packetLoss < 100 {
		return 1
	}

	return 2
}

func indexMetricValue(values []string, value string) float64 {
	for i, v := range values {
		if value == v {
			return float64(i)
		}
	}

	return 0
}

func addOutageFixture(st *Status) {
	activeAt := fixedNow.Add(2 * time.Hour)
	recentAt := fixedNow.Add(-1 * time.Hour)

	st.OutageReports = &OutageReports{
		Status:               true,
		AnyActive:            true,
		AnyActiveMaintenance: true,
		AnyRecent:            true,
		AnyRecentOutage:      true,
		ActiveList: []*OutageReport{
			{
				Id:            1001,
				BeginsAt:      activeAt,
				Duration:      90 * time.Minute,
				Type:          "planned_outage",
				State:         "announced",
				Impact:        "partial",
				EnSummary:     "Router replacement",
				EnDescription: "Planned router replacement.",
				AffectedEntities: []OutageEntity{
					{Name: "node", Id: 101, Label: "node1.prg"},
				},
			},
		},
		RecentList: []*OutageReport{
			{
				Id:            1002,
				BeginsAt:      recentAt,
				Duration:      30 * time.Minute,
				Type:          "unplanned_outage",
				State:         "resolved",
				Impact:        "full",
				EnSummary:     "Power failure",
				EnDescription: "Power failure was resolved.",
				AffectedEntities: []OutageEntity{
					{Name: "location", Id: 3, Label: "Praha"},
				},
			},
		},
	}
}

func writeNotice(t *testing.T, cfg *config.Config, text string) {
	t.Helper()

	if err := os.WriteFile(cfg.NoticeFile, []byte(text), 0o644); err != nil {
		t.Fatalf("write notice: %v", err)
	}

	if err := os.Chtimes(cfg.NoticeFile, fixedNoticeTime, fixedNoticeTime); err != nil {
		t.Fatalf("set notice time: %v", err)
	}
}

func getThroughRoutes(t *testing.T, app *application, target string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, target, nil)
	rr := httptest.NewRecorder()
	app.routes().ServeHTTP(rr, req)
	return rr
}

func requireStatus(t *testing.T, rr *httptest.ResponseRecorder, code int) {
	t.Helper()

	if rr.Code != code {
		t.Fatalf("expected HTTP %d, got %d: %s", code, rr.Code, rr.Body.String())
	}
}

func requireContains(t *testing.T, body string, substrings ...string) {
	t.Helper()

	for _, s := range substrings {
		if !strings.Contains(body, s) {
			t.Fatalf("expected body to contain %q\nbody:\n%s", s, body)
		}
	}
}

func requireNotContains(t *testing.T, body string, substrings ...string) {
	t.Helper()

	for _, s := range substrings {
		if strings.Contains(body, s) {
			t.Fatalf("expected body not to contain %q\nbody:\n%s", s, body)
		}
	}
}

func requireOccurrences(t *testing.T, body string, substring string, count int) {
	t.Helper()

	if got := strings.Count(body, substring); got != count {
		t.Fatalf("expected %q to occur %d times, got %d\nbody:\n%s", substring, count, got, body)
	}
}

func requireStatusCountsAfter(t *testing.T, body string, marker string, counts StatusCounts) {
	t.Helper()

	idx := strings.Index(body, marker)
	if idx == -1 {
		t.Fatalf("expected body to contain marker %q\nbody:\n%s", marker, body)
	}

	end := idx + 1200
	if end > len(body) {
		end = len(body)
	}
	segment := body[idx:end]
	expected := statusCountsMarkup(counts)
	if !strings.Contains(segment, expected) {
		t.Fatalf("expected status counts after %q to contain %q\nsegment:\n%s", marker, expected, segment)
	}
}

func statusCountsMarkup(counts StatusCounts) string {
	ret := fmt.Sprintf(
		`<span class="status-counts ms-1" aria-label="%d operational, %d degraded or under maintenance, %d down, %d total"><span class="text-success" title="Operational">%d</span>`,
		counts.Operational,
		counts.Degraded,
		counts.Down,
		counts.Total,
		counts.Operational,
	)
	if counts.Degraded > 0 {
		ret += fmt.Sprintf(`/<span class="text-warning" title="Degraded or under maintenance">%d</span>`, counts.Degraded)
	}
	if counts.Down > 0 {
		ret += fmt.Sprintf(`/<span class="text-danger" title="Down">%d</span>`, counts.Down)
	}
	ret += fmt.Sprintf(`/<span class="text-body" title="Total">%d</span></span>`, counts.Total)
	return ret
}

func requireMapKeys(t *testing.T, m map[string]any, keys ...string) {
	t.Helper()

	got := make([]string, 0, len(m))
	for k := range m {
		got = append(got, k)
	}
	sort.Strings(got)

	want := append([]string(nil), keys...)
	sort.Strings(want)

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("map keys = %v, want %v", got, want)
	}
}

func requireMapValue(t *testing.T, m map[string]any, key string) map[string]any {
	t.Helper()

	value, ok := m[key].(map[string]any)
	if !ok {
		t.Fatalf("%q = %#v, want object", key, m[key])
	}

	return value
}

func requireJSONString(t *testing.T, m map[string]any, key string, want string) {
	t.Helper()

	value, ok := m[key].(string)
	if !ok {
		t.Fatalf("%q = %#v, want string", key, m[key])
	}
	if value != want {
		t.Fatalf("%q = %q, want %q", key, value, want)
	}
}

func requireJSONBool(t *testing.T, m map[string]any, key string, want bool) {
	t.Helper()

	value, ok := m[key].(bool)
	if !ok {
		t.Fatalf("%q = %#v, want bool", key, m[key])
	}
	if value != want {
		t.Fatalf("%q = %v, want %v", key, value, want)
	}
}

func requireJSONNumber(t *testing.T, m map[string]any, key string, want float64) {
	t.Helper()

	value, ok := m[key].(float64)
	if !ok {
		t.Fatalf("%q = %#v, want number", key, m[key])
	}
	if value != want {
		t.Fatalf("%q = %v, want %v", key, value, want)
	}
}

func requireSliceValue(t *testing.T, m map[string]any, key string) []any {
	t.Helper()

	value, ok := m[key].([]any)
	if !ok {
		t.Fatalf("%q = %#v, want array", key, m[key])
	}

	return value
}

func requireSliceLength(t *testing.T, values []any, length int) {
	t.Helper()

	if len(values) != length {
		t.Fatalf("array length = %d, want %d", len(values), length)
	}
}

func requireSliceMap(t *testing.T, values []any, index int) map[string]any {
	t.Helper()

	if index >= len(values) {
		t.Fatalf("array length = %d, missing index %d", len(values), index)
	}

	value, ok := values[index].(map[string]any)
	if !ok {
		t.Fatalf("array[%d] = %#v, want object", index, values[index])
	}

	return value
}

func scrapeMetrics(t *testing.T, app *application) map[string]*dto.MetricFamily {
	t.Helper()

	rr := getThroughRoutes(t, app, "/metrics")
	requireStatus(t, rr, http.StatusOK)

	decoder := expfmt.NewDecoder(bytes.NewReader(rr.Body.Bytes()), expfmt.NewFormat(expfmt.TypeTextPlain))
	families := make(map[string]*dto.MetricFamily)

	for {
		family := &dto.MetricFamily{}
		err := decoder.Decode(family)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("parse metrics: %v\n%s", err, rr.Body.String())
		}

		families[family.GetName()] = family
	}

	return families
}

func requireGaugeFamily(t *testing.T, families map[string]*dto.MetricFamily, name string, help string) *dto.MetricFamily {
	t.Helper()

	family := families[name]
	if family == nil {
		t.Fatalf("missing metric family %s", name)
	}

	if got := family.GetHelp(); got != help {
		t.Fatalf("metric %s help = %q, want %q", name, got, help)
	}

	if got := family.GetType(); got != dto.MetricType_GAUGE {
		t.Fatalf("metric %s type = %s, want GAUGE", name, got)
	}

	return family
}

func requireMetricFamilies(t *testing.T, families map[string]*dto.MetricFamily, names ...string) {
	t.Helper()

	got := make([]string, 0, len(families))
	for name := range families {
		got = append(got, name)
	}
	sort.Strings(got)

	want := append([]string(nil), names...)
	sort.Strings(want)

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("metric families = %v, want %v", got, want)
	}
}

func requireMetricCount(t *testing.T, family *dto.MetricFamily, count int) {
	t.Helper()

	if got := len(family.GetMetric()); got != count {
		t.Fatalf("metric %s sample count = %d, want %d", family.GetName(), got, count)
	}
}

func requireMetricLabelSets(t *testing.T, family *dto.MetricFamily, labels ...map[string]string) {
	t.Helper()

	got := make([]map[string]string, 0, len(family.GetMetric()))
	for _, metric := range family.GetMetric() {
		got = append(got, metricLabels(metric))
	}

	for _, want := range labels {
		found := false
		for _, gotLabels := range got {
			if reflect.DeepEqual(gotLabels, want) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("metric %s missing labels %v; have %s", family.GetName(), want, describeMetricLabels(family))
		}
	}
}

func requireMetricValue(t *testing.T, family *dto.MetricFamily, labels map[string]string, value float64) {
	t.Helper()

	for _, metric := range family.GetMetric() {
		if reflect.DeepEqual(metricLabels(metric), labels) {
			if got := metric.GetGauge().GetValue(); got != value {
				t.Fatalf("metric %s%v = %v, want %v", family.GetName(), labels, got, value)
			}
			return
		}
	}

	t.Fatalf("metric %s with labels %v not found; have %s", family.GetName(), labels, describeMetricLabels(family))
}

func requireUnlabeledMetricValue(t *testing.T, family *dto.MetricFamily, value float64) {
	t.Helper()

	if len(family.GetMetric()) != 1 {
		t.Fatalf("metric %s has %d samples, want 1", family.GetName(), len(family.GetMetric()))
	}

	metric := family.GetMetric()[0]
	if labels := metricLabels(metric); len(labels) != 0 {
		t.Fatalf("metric %s labels = %v, want none", family.GetName(), labels)
	}

	if got := metric.GetGauge().GetValue(); got != value {
		t.Fatalf("metric %s = %v, want %v", family.GetName(), got, value)
	}
}

func metricLabels(metric *dto.Metric) map[string]string {
	ret := make(map[string]string)
	for _, label := range metric.GetLabel() {
		ret[label.GetName()] = label.GetValue()
	}
	return ret
}

func describeMetricLabels(family *dto.MetricFamily) string {
	var ret []string
	for _, metric := range family.GetMetric() {
		ret = append(ret, fmt.Sprintf("%v", metricLabels(metric)))
	}
	return strings.Join(ret, ", ")
}

func gaugeValue(t *testing.T, gauge prometheus.Gauge) float64 {
	t.Helper()

	metric := &dto.Metric{}
	if err := gauge.Write(metric); err != nil {
		t.Fatalf("write gauge: %v", err)
	}

	return metric.GetGauge().GetValue()
}
