package json

import (
	"encoding/json"
	"io"
	"time"
)

type Status struct {
	GeneratedAt   time.Time     `json:"generated_at"`
	Notice        string        `json:"notice"`
	VpsAdmin      VpsAdmin      `json:"vpsadmin"`
	OutageReports OutageReports `json:"outage_reports"`
	Locations     []Location    `json:"locations"`
	WebServices   []WebService  `json:"web_services"`
	NameServers   []NameServer  `json:"nameservers"`
}

type VpsAdmin struct {
	Api     bool `json:"api"`
	Console bool `json:"console"`
	Webui   bool `json:"webui"`
}

type OutageReports struct {
	Status    bool           `json:"status"`
	Announced []OutageReport `json:"announced"`
}

type OutageReport struct {
	Id            int64          `json:"id"`
	BeginsAt      time.Time      `json:"begins_at"`
	Duration      int            `json:"duration"`
	Planned       bool           `json:"planned"`
	State         string         `json:"state"`
	Type          string         `json:"type"`
	CsSummary     string         `json:"cs_summary"`
	CsDescription string         `json:"cs_description"`
	EnSummary     string         `json:"en_summary"`
	EnDescription string         `json:"en_description"`
	Entities      []OutageEntity `json:"entities"`
}

type OutageEntity struct {
	Name  string `json:"name"`
	Id    int64  `json:"id"`
	Label string `json:"label"`
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
	LocationId  int    `json:"location_id"`
	VpsAdmin    bool   `json:"vpsadmin"`
	Ping        bool   `json:"ping"`
	Maintenance bool   `json:"maintenance"`
}

type DnsResolver struct {
	Name   string `json:"name"`
	Ping   bool   `json:"ping"`
	Lookup bool   `json:"lookup"`
}

type WebService struct {
	Label       string `json:"label"`
	Description string `json:"description"`
	Url         string `json:"url"`
	Status      bool   `json:"status"`
}

type NameServer struct {
	Name   string `json:"name"`
	Ping   bool   `json:"ping"`
	Lookup bool   `json:"lookup"`
}

func ExportTo(w io.Writer, st *Status) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(st)
}
