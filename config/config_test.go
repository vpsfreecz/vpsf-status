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
	if cfg.CheckInterval != 30 {
		t.Fatalf("CheckInterval = %d, want 30", cfg.CheckInterval)
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

func TestParseConfigReturnsInvalidJSONError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"listen_address":`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := ParseConfig(path); err == nil {
		t.Fatal("ParseConfig returned nil error for invalid JSON")
	}
}

func TestParseConfigReturnsReadError(t *testing.T) {
	if _, err := ParseConfig(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Fatal("ParseConfig returned nil error for missing file")
	}
}
