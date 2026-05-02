package config

import (
	"encoding/json"
	"fmt"
	"os"
)

const DefaultHistoryDays = 90

type Config struct {
	ListenAddress string       `json:"listen_address"`
	DataDir       string       `json:"data_dir"`
	HistoryDir    string       `json:"history_dir"`
	HistoryDays   int          `json:"history_days"`
	NoticeFile    string       `json:"notice_file"`
	CheckInterval int          `json:"check_interval"`
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
	Label       string `json:"label"`
	Description string `json:"description"`
	Url         string `json:"url"`
	CheckUrl    string `json:"check_url"`
	Method      string `json:"method"`
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

	var cfg = Config{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	if cfg.NoticeFile == "" {
		cfg.NoticeFile = "notice.html"
	}

	if cfg.HistoryDir == "" {
		cfg.HistoryDir = "/var/lib/vpsf-status"
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

	return &cfg, nil
}
