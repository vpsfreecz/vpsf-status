package json

import (
	"encoding/json"
	"io"
	"time"
)

type Status struct {
	GeneratedAt time.Time  `json:"generated_at"`
	Notice      string     `json:"notice"`
	VpsAdmin    VpsAdmin   `json:"vpsadmin"`
	Locations   []Location `json:"locations"`
}

type VpsAdmin struct {
	Api     bool `json:"api"`
	Console bool `json:"console"`
	Webui   bool `json:"webui"`
}

type Location struct {
	Id           int           `json:"id"`
	Label        string        `json:"label"`
	Nodes        []Node        `json:"nodes"`
	DnsResolvers []DnsResolver `json:"dns_resolvers"`
}

type Node struct {
	Id          int    `json:"id"`
	Name        string `json:"name"`
	VpsAdmin    bool   `json:"vpsadmin"`
	Ping        bool   `json:"ping"`
	Maintenance bool   `json:"maintenance"`
}

type DnsResolver struct {
	Name   string `json:"name"`
	Ping   bool   `json:"ping"`
	Lookup bool   `json:"lookup"`
}

func ExportTo(w io.Writer, st *Status) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(st)
}
