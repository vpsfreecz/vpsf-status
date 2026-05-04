package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseConfigDefaultsAndFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	data := []byte(`{
		"listen_address": ":8080",
		"data_dir": ".",
		"vpsadmin": {
			"api_url": "https://api.vpsfree.cz",
			"webui_url": "https://vpsadmin.vpsfree.cz",
			"console_url": "https://console.vpsfree.cz/vzconsole.js"
		},
		"locations": [
			{
				"id": 3,
				"label": "Praha",
				"nodes": [
					{"id": 101, "name": "node1.prg", "ip_address": "172.16.0.10"}
				],
				"dns_resolvers": [
					{"name": "resolver-prg", "ip_address": "172.16.0.53"}
				]
			}
		],
		"web_services": [
			{
				"label": "vpsfree.cz",
				"description": "Website",
				"url": "https://vpsfree.cz",
				"check_url": "https://check.vpsfree.cz/",
				"method": "get"
			}
		],
		"nameservers": [
			{"name": "ns1.vpsfree.cz", "domain": "vpsfree.cz"}
		]
	}`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := ParseConfig(path)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	if cfg.NoticeFile != "notice.html" {
		t.Fatalf("NoticeFile = %q, want notice.html", cfg.NoticeFile)
	}
	if cfg.StateDir != "/var/lib/vpsf-status" {
		t.Fatalf("StateDir = %q, want /var/lib/vpsf-status", cfg.StateDir)
	}
	if cfg.HistoryDays != DefaultHistoryDays {
		t.Fatalf("HistoryDays = %d, want %d", cfg.HistoryDays, DefaultHistoryDays)
	}
	if cfg.CheckInterval != 30 {
		t.Fatalf("CheckInterval = %d, want 30", cfg.CheckInterval)
	}
	if cfg.CheckTimeout != DefaultCheckTimeout {
		t.Fatalf("CheckTimeout = %d, want %d", cfg.CheckTimeout, DefaultCheckTimeout)
	}
	if cfg.VpsAdmin.ApiUrl != "https://api.vpsfree.cz" || cfg.Locations[0].Nodes[0].Name != "node1.prg" {
		t.Fatalf("parsed config = %+v", cfg)
	}
	if cfg.WebServices[0].CheckUrl != "https://check.vpsfree.cz/" || cfg.WebServices[0].Method != "get" {
		t.Fatalf("web service = %+v", cfg.WebServices[0])
	}
	if cfg.NameServers[0].Name != "ns1.vpsfree.cz" || cfg.NameServers[0].Domain != "vpsfree.cz" {
		t.Fatalf("nameserver = %+v", cfg.NameServers[0])
	}
}

func TestParseConfigSample(t *testing.T) {
	cfg, err := ParseConfig(filepath.Join("..", "config-sample.json"))
	if err != nil {
		t.Fatalf("ParseConfig sample: %v", err)
	}

	if cfg.ListenAddress != ":8080" || cfg.DataDir != "." || cfg.NoticeFile != "notice.html" || cfg.StateDir != "/tmp/vpsf-status" || cfg.HistoryDays != 90 || cfg.CheckTimeout != 60 {
		t.Fatalf("sample paths/listen values = %+v", cfg)
	}
	if cfg.VpsAdmin.ApiUrl == "" || cfg.VpsAdmin.WebuiUrl == "" || cfg.VpsAdmin.ConsoleUrl == "" {
		t.Fatalf("sample vpsAdmin URLs = %+v", cfg.VpsAdmin)
	}
	if len(cfg.Locations) != 2 || len(cfg.WebServices) != 3 || len(cfg.NameServers) != 2 {
		t.Fatalf("sample config counts = locations:%d web:%d nameservers:%d", len(cfg.Locations), len(cfg.WebServices), len(cfg.NameServers))
	}
	if cfg.Locations[0].Nodes[0].Id != 120 || cfg.Locations[0].Nodes[0].Name != "node19.prg" {
		t.Fatalf("sample first node = %+v", cfg.Locations[0].Nodes[0])
	}
	if len(cfg.Locations[1].DnsResolvers) != 2 || cfg.Locations[1].DnsResolvers[0].Name != "ns1.brq.vpsfree.cz" || cfg.Locations[1].DnsResolvers[0].IpAddress != "37.205.11.200" {
		t.Fatalf("sample Brno resolvers = %+v", cfg.Locations[1].DnsResolvers)
	}
}

func TestParseConfigReturnsInvalidJSONError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"listen_address":`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := ParseConfig(path); err == nil {
		t.Fatal("ParseConfig returned nil error for invalid JSON")
	}
}

func TestParseConfigRejectsHistoryDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"history_dir": "/tmp/vpsf-status-history"}`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := ParseConfig(path); err == nil {
		t.Fatal("ParseConfig returned nil error for history_dir")
	}
}

func TestParseConfigPreservesHistoryDays(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"history_days": 180}`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := ParseConfig(path)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	if cfg.HistoryDays != 180 {
		t.Fatalf("HistoryDays = %d, want 180", cfg.HistoryDays)
	}
}

func TestParseConfigPreservesCheckTimeout(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"check_timeout": 45}`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := ParseConfig(path)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	if cfg.CheckTimeout != 45 {
		t.Fatalf("CheckTimeout = %d, want 45", cfg.CheckTimeout)
	}
}

func TestParseConfigRejectsNegativeCheckTimeout(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"check_timeout": -1}`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := ParseConfig(path); err == nil {
		t.Fatal("ParseConfig returned nil error for negative check_timeout")
	}
}

func TestParseConfigRejectsNegativeHistoryDays(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"history_days": -1}`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := ParseConfig(path); err == nil {
		t.Fatal("ParseConfig returned nil error for negative history_days")
	}
}

func TestParseConfigReturnsReadError(t *testing.T) {
	if _, err := ParseConfig(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Fatal("ParseConfig returned nil error for missing file")
	}
}
