package json

import (
	"encoding/json"
	"io"
	"time"
)

type Status struct {
	GeneratedAt   time.Time     `json:"generated_at"`
	Notice        Notice        `json:"notice"`
	VpsAdmin      VpsAdmin      `json:"vpsadmin"`
	OutageReports OutageReports `json:"outage_reports"`
	Locations     []Location    `json:"locations"`
	WebServices   []WebService  `json:"web_services"`
	NameServers   []NameServer  `json:"nameservers"`
}

type Notice struct {
	Any       bool      `json:"any"`
	Text      string    `json:"text"`
	UpdatedAt time.Time `json:"updated_at"`
}

type VpsAdmin struct {
	Api     WebService `json:"api"`
	Console WebService `json:"console"`
	Webui   WebService `json:"webui"`
}

type OutageReports struct {
	Status    bool           `json:"status"`
	Announced []OutageReport `json:"announced"`
	Recent    []OutageReport `json:"recent"`
}

type OutageReport struct {
	Id            int64          `json:"id"`
	BeginsAt      time.Time      `json:"begins_at"`
	Duration      int            `json:"duration"`
	Type          string         `json:"type"`
	State         string         `json:"state"`
	Impact        string         `json:"impact"`
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
	Id              int     `json:"id"`
	Name            string  `json:"name"`
	LocationId      int     `json:"location_id"`
	OsType          string  `json:"os_type"`
	VpsAdmin        bool    `json:"vpsadmin"`
	Ping            string  `json:"ping"`
	Maintenance     bool    `json:"maintenance"`
	PoolState       string  `json:"pool_state"`
	PoolScan        string  `json:"pool_scan"`
	PoolScanPercent float64 `json:"pool_scan_percent"`
	PoolStatus      bool    `json:"pool_status"`
}

type DnsResolver struct {
	Name   string `json:"name"`
	Ping   string `json:"ping"`
	Lookup bool   `json:"lookup"`
}

type WebService struct {
	Label       string `json:"label"`
	Description string `json:"description"`
	Url         string `json:"url"`
	Status      string `json:"status"`
}

type NameServer struct {
	Name   string `json:"name"`
	Ping   string `json:"ping"`
	Lookup bool   `json:"lookup"`
}

func ExportTo(w io.Writer, st *Status) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(st)
}
