package main

import (
	"testing"

	"github.com/vpsfreecz/vpsf-status/config"
)

func TestOpenConfigRuntimeDefaults(t *testing.T) {
	cfg := &config.Config{
		VpsAdmin: config.VpsAdmin{
			ApiUrl:     "https://api.example",
			WebuiUrl:   "https://webui.example",
			ConsoleUrl: "https://console.example",
		},
		Locations: []config.Location{
			{
				Id:    3,
				Label: "Praha",
				Nodes: []config.Node{
					{Id: 101, Name: "node1.prg", IpAddress: "192.0.2.10"},
				},
				DnsResolvers: []config.DnsResolver{
					{Name: "resolver-prg", IpAddress: "192.0.2.53"},
				},
			},
		},
		WebServices: []config.WebService{
			{
				Label:        "defaulted",
				Description:  "Default description",
				Descriptions: map[string]string{"cs": "Výchozí popis"},
				Url:          "https://defaulted.example",
			},
			{
				Label:    "custom",
				Url:      "https://custom.example",
				CheckUrl: "https://custom.example/status",
				Method:   "get",
			},
		},
		NameServers: []config.NameServer{
			{Name: "ns1.example", Domain: "example.org"},
		},
	}

	st := openConfig(cfg)

	if st.VpsAdmin.Api.CheckUrl != "https://api.example" || st.VpsAdmin.Webui.CheckUrl != "https://webui.example" || st.VpsAdmin.Console.CheckUrl != "https://console.example" {
		t.Fatalf("vpsAdmin check URLs = api:%q webui:%q console:%q", st.VpsAdmin.Api.CheckUrl, st.VpsAdmin.Webui.CheckUrl, st.VpsAdmin.Console.CheckUrl)
	}

	defaulted := st.Services.Web[0]
	if defaulted.CheckUrl != defaulted.Url {
		t.Fatalf("default CheckUrl = %q, want %q", defaulted.CheckUrl, defaulted.Url)
	}
	if defaulted.Method != "head" {
		t.Fatalf("default Method = %q, want head", defaulted.Method)
	}
	if got := defaulted.DescriptionForLocale("cs"); got != "Výchozí popis" {
		t.Fatalf("Czech description = %q, want Výchozí popis", got)
	}
	if got := defaulted.DescriptionForLocale("de"); got != "Default description" {
		t.Fatalf("fallback description = %q, want Default description", got)
	}

	custom := st.Services.Web[1]
	if custom.CheckUrl != "https://custom.example/status" || custom.Method != "get" {
		t.Fatalf("custom web service = %+v", custom)
	}

	resolver := st.LocationList[0].DnsResolverList[0]
	if resolver.ResolveDomain != "www.google.com" || resolver.IpAddress != "192.0.2.53" {
		t.Fatalf("resolver = %+v", resolver)
	}
	if resolver.Ping.Name != "resolver-prg" || resolver.Ping.IpAddress != "192.0.2.53" {
		t.Fatalf("resolver ping = %+v", resolver.Ping)
	}

	nameServer := st.Services.NameServer[0]
	if nameServer.IpAddress != "ns1.example" || nameServer.ResolveDomain != "example.org" {
		t.Fatalf("nameserver = %+v", nameServer)
	}
	if nameServer.Ping.Name != "ns1.example" || nameServer.Ping.IpAddress != "ns1.example" {
		t.Fatalf("nameserver ping = %+v", nameServer.Ping)
	}
}
