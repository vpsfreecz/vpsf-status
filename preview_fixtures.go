//go:build preview

package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/vpsfreecz/vpsf-status/config"
)

var previewNow = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)

func newPreviewApplication(dataDir, stateDir, scenario string) (*application, error) {
	switch scenario {
	case "mixed", "healthy", "loading", "fetch-failure":
	default:
		return nil, fmt.Errorf("unknown preview scenario %q (choose mixed, healthy, loading or fetch-failure)", scenario)
	}

	cfg := &config.Config{
		DataDir: dataDir, StateDir: stateDir, HistoryDays: 90, CheckInterval: 30,
		NoticeFile: filepath.Join(stateDir, "notice.html"),
		VpsAdmin: config.VpsAdmin{
			ApiUrl: "https://api.example.invalid", WebuiUrl: "https://admin.example.invalid", ConsoleUrl: "https://console.example.invalid",
		},
		Locations: []config.Location{
			{Id: 1, Label: "Praha: sample location / ukázková lokalita", Nodes: []config.Node{
				{Id: 101, Name: "node01.preview.example.invalid", IpAddress: "192.0.2.11"},
				{Id: 102, Name: "node02-maintenance.preview.example.invalid", IpAddress: "192.0.2.12"},
				{Id: 103, Name: "node03-with-a-long-hostname-for-responsive-layout.preview.example.invalid", IpAddress: "192.0.2.13"},
			}, DnsResolvers: []config.DnsResolver{{Name: "resolver01.preview.example.invalid", IpAddress: "192.0.2.53"}}},
			{Id: 2, Label: "Brno: preview", Nodes: []config.Node{{Id: 201, Name: "node04.preview.example.invalid", IpAddress: "198.51.100.11"}}},
		},
		WebServices: []config.WebService{
			{Label: "www.preview.example.invalid", Url: "https://www.example.invalid", Description: "Synthetic public website", Descriptions: map[string]string{"cs": "Ukázkové veřejné webové stránky"}},
			{Label: "knowledge-base-with-a-long-service-label.preview.example.invalid", Url: "https://kb.example.invalid", Description: "Synthetic knowledge base with a longer description to check wrapping", Descriptions: map[string]string{"cs": "Ukázková znalostní báze s delším popisem pro kontrolu zalamování textu"}},
		},
		NameServers: []config.NameServer{{Name: "ns01.preview.example.invalid", Domain: "example.invalid"}},
	}
	st := openConfig(cfg) // Constructs state only; never start Status.initialize.
	history, err := openHistoryStore(stateDir)
	if err != nil {
		return nil, err
	}
	st.History = history
	app := &application{config: cfg, status: st, now: func() time.Time { return previewNow }}
	ready := false
	defer func() {
		if !ready {
			history.Close()
		}
	}()

	setPreviewOperational(st)
	if scenario == "mixed" {
		st.LocationList[0].NodeList[1].ApiMaintenance = true
		st.LocationList[0].NodeList[2].Ping.PacketLoss = 35
		st.LocationList[1].NodeList[0].Ping.PacketLoss = 100
		st.Services.Web[1].Status = false
		st.Services.Web[1].StatusCode = http.StatusServiceUnavailable
		st.VpsAdmin.Console.Status = false
		st.VpsAdmin.Console.Maintenance = true
		st.OutageReports = createCurrentOutageReports(previewOutages(), previewNow)
		st.SecurityAdvisories = createCurrentSecurityAdvisories([]*SecurityAdvisory{{
			Id: 1, PublishedAt: previewNow.Add(-6 * time.Hour), UpdatedAt: previewNow.Add(-time.Hour), State: "announced",
			Name: "Synthetic security advisory / Ukázkové bezpečnostní upozornění", AffectedNodeCount: 2,
			EnSummary: "Example advisory with a longer explanation: the preview nodes need a scheduled restart after a fictional security update.",
			CsSummary: "Ukázkové upozornění s delším popisem: ukázkové nody čekají na plánovaný restart po smyšlené bezpečnostní aktualizaci.",
		}})
	}
	if scenario == "fetch-failure" {
		st.OutageReports.Status = false
		st.SecurityAdvisories.Status = false
	}
	if scenario == "loading" {
		st.Initialized = false
		st.Exporter.up.Set(0)
	}

	if err := seedPreviewHistory(st, scenario); err != nil {
		return nil, err
	}
	if err := os.WriteFile(cfg.NoticeFile, []byte("<p>Sample data / Testovací data. Monitoring is not running / Monitorování není spuštěno.</p>"), 0o600); err != nil {
		return nil, err
	}
	if err := os.Chtimes(cfg.NoticeFile, previewNow, previewNow); err != nil {
		return nil, err
	}
	st.Exporter.notice.Set(1)
	if err := app.parseTemplates(); err != nil {
		return nil, err
	}
	if err := app.refreshIndexBodies(previewNow, true); err != nil {
		return nil, err
	}
	ready = true
	return app, nil
}

func setPreviewOperational(st *Status) {
	st.Initialized = true
	st.Exporter.up.Set(1)
	st.OutageReports.Status = true
	st.SecurityAdvisories.Status = true
	for _, ws := range []*WebService{st.VpsAdmin.Api, st.VpsAdmin.Webui, st.VpsAdmin.Console} {
		ws.Status, ws.StatusCode, ws.LastCheck = true, http.StatusOK, previewNow
	}
	for _, loc := range st.LocationList {
		for _, node := range loc.NodeList {
			node.OsType, node.ApiStatus, node.LastApiCheck = "vpsadminos", true, previewNow
			node.PoolState, node.PoolScan, node.PoolStatus = "online", "none", true
			node.Ping.LastCheck = previewNow
		}
		for _, dns := range loc.DnsResolverList {
			dns.ResolveDomain = "example.invalid"
			dns.ResolveStatus, dns.LastResolveCheck, dns.Ping.LastCheck = true, previewNow, previewNow
		}
	}
	for _, ws := range st.Services.Web {
		ws.Status, ws.StatusCode, ws.LastCheck = true, http.StatusOK, previewNow
	}
	for _, ns := range st.Services.NameServer {
		ns.ResolveStatus, ns.LastResolveCheck, ns.Ping.LastCheck = true, previewNow, previewNow
	}
}

func previewOutages() []*OutageReport {
	return []*OutageReport{
		{Id: 1, BeginsAt: previewNow.Add(-time.Hour), Duration: 2 * time.Hour, Type: outageTypePlanned, State: "announced", Impact: "network",
			EnSummary:        "Synthetic maintenance: network equipment replacement with a long summary to check phone layouts",
			CsSummary:        "Ukázková údržba: výměna síťového vybavení s delším popisem pro kontrolu zobrazení na telefonu",
			AffectedEntities: []OutageEntity{{Name: "Node", EntityType: "node", Id: 102, Label: "node02-maintenance.preview.example.invalid"}}},
		{Id: 2, BeginsAt: previewNow.Add(-30 * time.Minute), Duration: time.Hour, Type: outageTypeUnplanned, State: "announced", Impact: "unavailability",
			EnSummary: "Synthetic connectivity outage", CsSummary: "Ukázkový výpadek připojení",
			AffectedEntities: []OutageEntity{{Name: "Node", EntityType: "node", Id: 201, Label: "node04.preview.example.invalid"}}},
		{Id: 3, BeginsAt: previewNow.Add(-24 * time.Hour), FinishedAt: previewNow.Add(-23 * time.Hour), Duration: time.Hour, Type: outageTypeUnplanned, State: "finished", Impact: "unavailability",
			EnSummary: "Resolved synthetic outage", CsSummary: "Vyřešený ukázkový výpadek",
			AffectedEntities: []OutageEntity{{Name: "Node", EntityType: "node", Id: 101, Label: "node01.preview.example.invalid"}}},
	}
}

func seedPreviewHistory(st *Status, scenario string) error {
	if err := recordConfiguredEntitySnapshots(st, previewNow); err != nil {
		return err
	}
	if err := st.History.ReplaceOutages(append(st.OutageReports.ActiveList, st.OutageReports.RecentList...), previewNow); err != nil {
		return err
	}
	for _, entity := range configuredHistoryEntities(st) {
		method, _ := availabilityProbeMethod(entity.Kind)
		target := ProbeTarget{EntityKind: entity.Kind, EntityID: entity.ID, EntityLabel: entity.Label, Method: method}
		if err := st.History.RecordProbeStatus(target, historyProbeStateOperational, "Synthetic probe / Ukázková kontrola", previewNow.AddDate(0, 0, -90)); err != nil {
			return err
		}
		if scenario != "mixed" {
			continue
		}
		if entity.Kind == historyEntityNode && entity.ID == "node01.preview.example.invalid" {
			// More than one real event-log page, with several history/popover states.
			states := []string{historyProbeStateDown, historyProbeStateOperational, historyProbeStateMaintenance, historyProbeStateOperational, historyProbeStateDegraded, historyProbeStateOperational}
			for i := 0; i < 60; i++ {
				at := previewNow.Add(-time.Duration(60-i) * 12 * time.Hour)
				if err := st.History.RecordProbeStatus(target, states[i%len(states)], "Synthetic event with a longer diagnostic message / Ukázková událost s delší diagnostickou zprávou", at); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
