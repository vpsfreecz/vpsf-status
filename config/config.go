package config

import (
	"encoding/json"
	"io/ioutil"
	"os"
)

type Config struct {
	ListenAddress string       `json:"listen_address"`
	DataDir       string       `json:"data_dir"`
	NoticeFile    string       `json:"notice_file"`
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
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	byteResult, _ := ioutil.ReadAll(file)

	var cfg = Config{}
	json.Unmarshal([]byte(byteResult), &cfg)

	if cfg.NoticeFile == "" {
		cfg.NoticeFile = "notice.html"
	}

	return &cfg, nil
}
