package config

import (
	"encoding/json"
	"io/ioutil"
	"os"
)

type Config struct {
	ListenAddress string     `json:"listen_address"`
	StateDir      string     `json:"state_dir"`
	VpsAdmin      VpsAdmin   `json:"vpsadmin"`
	Locations     []Location `json:"locations"`
}

type VpsAdmin struct {
	ApiUrl     string `json:"api_url"`
	WebuiUrl   string `json:"webui_url"`
	ConsoleUrl string `json:"console_url"`
}

type Location struct {
	Id int `json:"id"`
	Label string `json:"label"`
	Nodes []Node `json:"nodes"`
	DnsResolvers []DnsResolver `json:"dns_resolvers"`
}

type Node struct {
	Name      string `json:"name"`
	Id        int    `json:"id"`
	IpAddress string `json:"ip_address"`
}

type DnsResolver struct {
	Name string `json:"name"`
	IpAddress string `json:"ip_address"`
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
	return &cfg, nil
}
