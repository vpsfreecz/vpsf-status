package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func BenchmarkRouteIndexCold(b *testing.B) {
	benchmarkRoute(b, "/", false)
}

func BenchmarkRouteIndexWarm(b *testing.B) {
	benchmarkRoute(b, "/", true)
}

func BenchmarkRouteEntityCold(b *testing.B) {
	benchmarkRoute(b, "/entity?kind=node&id=node1.prg", false)
}

func BenchmarkRouteEntityWarm(b *testing.B) {
	benchmarkRoute(b, "/entity?kind=node&id=node1.prg", true)
}

func BenchmarkRouteGroupCold(b *testing.B) {
	benchmarkRoute(b, "/group?kind=location&id=3", false)
}

func BenchmarkRouteGroupWarm(b *testing.B) {
	benchmarkRoute(b, "/group?kind=location&id=3", true)
}

func BenchmarkRouteJSONCold(b *testing.B) {
	benchmarkRoute(b, "/json", false)
}

func BenchmarkRouteJSONWarm(b *testing.B) {
	benchmarkRoute(b, "/json", true)
}

func BenchmarkRouteMetrics(b *testing.B) {
	benchmarkRoute(b, "/metrics", true)
}

func BenchmarkRouteAbout(b *testing.B) {
	benchmarkRoute(b, "/about", true)
}

func BenchmarkRouteStatic(b *testing.B) {
	benchmarkRoute(b, "/static/favicon.png", true)
}

func benchmarkRoute(b *testing.B, target string, warm bool) {
	now := fixedNow
	app, st, _ := newTestApplication(b)
	app.now = func() time.Time {
		return now
	}
	setOperationalFixture(st)
	seedBenchmarkHistory(b, st)

	handler := app.routes()
	req := httptest.NewRequest(http.MethodGet, target, nil)

	if warm {
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req.Clone(req.Context()))
		if rr.Code != http.StatusOK {
			b.Fatalf("warmup status = %d", rr.Code)
		}
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !warm {
			app.responseCache = newResponseCache()
			now = now.Add(2 * time.Second)
		}

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req.Clone(req.Context()))
		if rr.Code != http.StatusOK {
			b.Fatalf("status = %d", rr.Code)
		}
	}
}

func seedBenchmarkHistory(b *testing.B, st *Status) {
	b.Helper()

	targets := []ProbeTarget{
		{EntityKind: historyEntityVpsAdmin, EntityID: "webui", EntityLabel: "vpsAdmin web UI", Method: "HTTP"},
		{EntityKind: historyEntityVpsAdmin, EntityID: "api", EntityLabel: "vpsAdmin API", Method: "HTTP"},
		{EntityKind: historyEntityVpsAdmin, EntityID: "console", EntityLabel: "Remote Console", Method: "HTTP"},
	}
	for _, loc := range st.LocationList {
		for _, node := range loc.NodeList {
			targets = append(targets, ProbeTarget{EntityKind: historyEntityNode, EntityID: node.Name, EntityLabel: node.Name, Method: "Ping"})
		}
		for _, resolver := range loc.DnsResolverList {
			targets = append(targets, ProbeTarget{EntityKind: historyEntityDnsResolver, EntityID: resolver.Name, EntityLabel: resolver.Name, Method: "Lookup"})
		}
	}
	for _, ws := range st.Services.Web {
		targets = append(targets, ProbeTarget{EntityKind: historyEntityWebService, EntityID: ws.Label, EntityLabel: ws.Label, Method: "HTTP"})
	}
	for _, ns := range st.Services.NameServer {
		targets = append(targets, ProbeTarget{EntityKind: historyEntityNameServer, EntityID: ns.Name, EntityLabel: ns.Name, Method: "Lookup"})
	}

	for _, target := range targets {
		if err := st.History.RecordProbeStatus(target, historyProbeStateOperational, "ok", fixedNow.AddDate(-1, 0, 0).Add(-time.Hour)); err != nil {
			b.Fatalf("record probe status: %v", err)
		}
	}
}
