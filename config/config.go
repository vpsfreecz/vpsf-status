package config

import (
	"encoding/json"
	"fmt"
	"os"
)

const (
	DefaultHistoryDays  = 90
	DefaultCheckTimeout = 60
)

type Config struct {
	ListenAddress string       `json:"listen_address"`
	DataDir       string       `json:"data_dir"`
	StateDir      string       `json:"state_dir"`
	HistoryDays   int          `json:"history_days"`
	NoticeFile    string       `json:"notice_file"`
	CheckInterval int          `json:"check_interval"`
	CheckTimeout  int          `json:"check_timeout"`
	VpsAdmin      VpsAdmin     `json:"vpsadmin"`
	Locations     []Location   `json:"locations"`
	WebServices   []WebService `json:"web_services"`
	NameServers   []NameServer `json:"nameservers"`
}

type VpsAdmin struct {
	ApiUrl     string `json:"api_url"`
	WebuiUrl   string `json:"webui_url"`
	ConsoleUrl string `json:"console_url"`
}

type Location struct {
	Id           int           `json:"id"`
	Label        string        `json:"label"`
	Nodes        []Node        `json:"nodes"`
	DnsResolvers []DnsResolver `json:"dns_resolvers"`
}

type Node struct {
	Name      string `json:"name"`
	Id        int    `json:"id"`
	IpAddress string `json:"ip_address"`
}

type DnsResolver struct {
	Name      string `json:"name"`
	IpAddress string `json:"ip_address"`
}

type WebService struct {
	Label        string            `json:"label"`
	Description  string            `json:"description"`
	Descriptions map[string]string `json:"descriptions"`
	Url          string            `json:"url"`
	CheckUrl     string            `json:"check_url"`
	Method       string            `json:"method"`
}

type NameServer struct {
	Name   string
	Domain string
}

func ParseConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var keys map[string]json.RawMessage
	if err := json.Unmarshal(data, &keys); err != nil {
		return nil, err
	}
	if _, ok := keys["history_dir"]; ok {
		return nil, fmt.Errorf("history_dir was renamed to state_dir")
	}

	var cfg = Config{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	if cfg.NoticeFile == "" {
		cfg.NoticeFile = "notice.html"
	}

	if cfg.StateDir == "" {
		cfg.StateDir = "/var/lib/vpsf-status"
	}

	if cfg.HistoryDays < 0 {
		return nil, fmt.Errorf("history_days must not be negative")
	}
	if cfg.HistoryDays == 0 {
		cfg.HistoryDays = DefaultHistoryDays
	}

	if cfg.CheckInterval == 0 {
		cfg.CheckInterval = 30
	}
	if cfg.CheckTimeout < 0 {
		return nil, fmt.Errorf("check_timeout must not be negative")
	}
	if cfg.CheckTimeout == 0 {
		cfg.CheckTimeout = DefaultCheckTimeout
	}

	return &cfg, nil
}
