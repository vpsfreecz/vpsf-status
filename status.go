package main

import (
	"time"

	"github.com/vpsfreecz/vpsf-status/config"
	"github.com/vpsfreecz/vpsf-status/json"
)

type Status struct {
	VpsAdmin      VpsAdmin
	OutageReports *OutageReports
	LocationList  []*Location
	LocationMap   map[string]*Location
	GlobalNodeMap map[string]*Node
	Services      *Services
}

type VpsAdmin struct {
	Api     *WebService
	Webui   *WebService
	Console *WebService
}

type OutageReports struct {
	Status         bool
	List           []*OutageReport
	Any            bool
	AnyMaintenance bool
	AnyOutage      bool
}

type OutageReport struct {
	Id               int64
	BeginsAt         time.Time
	Duration         time.Duration
	Planned          bool
	State            string
	Type             string
	CsSummary        string
	CsDescription    string
	EnSummary        string
	EnDescription    string
	AffectedEntities []OutageEntity
}

type OutageEntity struct {
	Name  string
	Id    int64
	Label string
}

type Location struct {
	Id              int
	Label           string
	NodeList        []*Node
	NodeMap         map[string]*Node
	DnsResolverList []*DnsResolver
}

type Node struct {
	Id         int
	Name       string
	LocationId int
	IpAddress  string

	ApiStatus      bool
	ApiMaintenance bool
	LastApiCheck   time.Time

	Ping *PingCheck
}

type DnsResolver struct {
	Name          string
	IpAddress     string
	ResolveDomain string

	ResolveStatus    bool
	LastResolveCheck time.Time

	Ping *PingCheck
}

type Services struct {
	Web        []*WebService
	NameServer []*DnsResolver
}

type WebService struct {
	Label        string
	Description  string
	Url          string
	CheckUrl     string
	Method       string
	Status       bool
	StatusCode   int
	StatusString string
	LastCheck    time.Time
}

type PingCheck struct {
	Name       string
	IpAddress  string
	PacketLoss float64
	LastCheck  time.Time
}

func openConfig(cfg *config.Config) *Status {
	st := Status{
		LocationList:  make([]*Location, len(cfg.Locations)),
		LocationMap:   make(map[string]*Location),
		GlobalNodeMap: make(map[string]*Node),
		Services: &Services{
			Web:        make([]*WebService, len(cfg.WebServices)),
			NameServer: make([]*DnsResolver, len(cfg.NameServers)),
		},
		OutageReports: &OutageReports{
			List: make([]*OutageReport, 0),
		},
	}

	st.VpsAdmin.Api = &WebService{
		Label:    cfg.VpsAdmin.ApiUrl,
		Url:      cfg.VpsAdmin.ApiUrl,
		CheckUrl: cfg.VpsAdmin.ApiUrl,
	}
	st.VpsAdmin.Webui = &WebService{
		Label:    cfg.VpsAdmin.WebuiUrl,
		Url:      cfg.VpsAdmin.WebuiUrl,
		CheckUrl: cfg.VpsAdmin.WebuiUrl,
	}
	st.VpsAdmin.Console = &WebService{
		Label:    "Console Router",
		Url:      cfg.VpsAdmin.ConsoleUrl,
		CheckUrl: cfg.VpsAdmin.ConsoleUrl,
	}

	for iLoc, cfgLoc := range cfg.Locations {

		loc := Location{
			Id:              cfgLoc.Id,
			Label:           cfgLoc.Label,
			NodeList:        make([]*Node, len(cfgLoc.Nodes)),
			NodeMap:         make(map[string]*Node),
			DnsResolverList: make([]*DnsResolver, len(cfgLoc.DnsResolvers)),
		}

		for iNode, cfgNode := range cfgLoc.Nodes {
			n := Node{
				Id:         cfgNode.Id,
				Name:       cfgNode.Name,
				LocationId: cfgLoc.Id,
				IpAddress:  cfgNode.IpAddress,
				Ping: &PingCheck{
					Name:      cfgNode.Name,
					IpAddress: cfgNode.IpAddress,
				},
			}

			loc.NodeList[iNode] = &n
			loc.NodeMap[cfgNode.Name] = &n
			st.GlobalNodeMap[cfgNode.Name] = &n
		}

		for iDns, cfgDns := range cfgLoc.DnsResolvers {
			loc.DnsResolverList[iDns] = &DnsResolver{
				Name:          cfgDns.Name,
				IpAddress:     cfgDns.IpAddress,
				ResolveDomain: "www.google.com",
				Ping: &PingCheck{
					Name:      cfgDns.Name,
					IpAddress: cfgDns.IpAddress,
				},
			}
		}

		st.LocationList[iLoc] = &loc
		st.LocationMap[loc.Label] = &loc
	}

	for iWs, cfgWs := range cfg.WebServices {
		ws := WebService{
			Label:       cfgWs.Label,
			Description: cfgWs.Description,
			Url:         cfgWs.Url,
			CheckUrl:    cfgWs.CheckUrl,
			Method:      cfgWs.Method,
		}

		if ws.CheckUrl == "" {
			ws.CheckUrl = ws.Url
		}

		if ws.Method == "" {
			ws.Method = "head"
		}

		st.Services.Web[iWs] = &ws
	}

	for iNs, cfgNs := range cfg.NameServers {
		ns := DnsResolver{
			Name:          cfgNs.Name,
			IpAddress:     cfgNs.Name,
			ResolveDomain: cfgNs.Domain,
			Ping: &PingCheck{
				Name:      cfgNs.Name,
				IpAddress: cfgNs.Name,
			},
		}

		st.Services.NameServer[iNs] = &ns
	}

	return &st
}

func (st *Status) ToJson(now time.Time, notice string) *json.Status {
	outages := st.OutageReports

	ret := &json.Status{
		VpsAdmin: json.VpsAdmin{
			Api:     st.VpsAdmin.Api.Status,
			Console: st.VpsAdmin.Console.Status,
			Webui:   st.VpsAdmin.Webui.Status,
		},
		OutageReports: json.OutageReports{
			Status:    outages.Status,
			Announced: make([]json.OutageReport, len(outages.List)),
		},
		Locations:   make([]json.Location, len(st.LocationList)),
		WebServices: make([]json.WebService, len(st.Services.Web)),
		NameServers: make([]json.NameServer, len(st.Services.NameServer)),
		Notice:      notice,
		GeneratedAt: now,
	}

	for iOutage, outage := range outages.List {
		jsonOutage := json.OutageReport{
			Id:            outage.Id,
			BeginsAt:      outage.BeginsAt,
			Duration:      int(outage.Duration.Minutes()),
			Planned:       outage.Planned,
			State:         outage.State,
			Type:          outage.Type,
			CsSummary:     outage.CsSummary,
			CsDescription: outage.CsDescription,
			EnSummary:     outage.EnSummary,
			EnDescription: outage.EnDescription,
			Entities:      make([]json.OutageEntity, len(outage.AffectedEntities)),
		}

		for iEnt, ent := range outage.AffectedEntities {
			jsonOutage.Entities[iEnt] = json.OutageEntity{
				Name:  ent.Name,
				Id:    ent.Id,
				Label: ent.Label,
			}
		}

		ret.OutageReports.Announced[iOutage] = jsonOutage
	}

	for iLoc, loc := range st.LocationList {
		jsonLoc := json.Location{
			Id:           loc.Id,
			Label:        loc.Label,
			Nodes:        make([]json.Node, len(loc.NodeList)),
			DnsResolvers: make([]json.DnsResolver, len(loc.DnsResolverList)),
		}

		for iNode, node := range loc.NodeList {
			jsonLoc.Nodes[iNode] = json.Node{
				Id:          node.Id,
				Name:        node.Name,
				LocationId:  node.LocationId,
				VpsAdmin:    node.ApiStatus,
				Ping:        node.Ping.PacketLoss <= 20,
				Maintenance: node.ApiMaintenance,
			}
		}

		for iDns, dns := range loc.DnsResolverList {
			jsonLoc.DnsResolvers[iDns] = json.DnsResolver{
				Name:   dns.Name,
				Ping:   dns.Ping.PacketLoss <= 20,
				Lookup: dns.ResolveStatus,
			}
		}

		ret.Locations[iLoc] = jsonLoc
	}

	for iWs, ws := range st.Services.Web {
		ret.WebServices[iWs] = json.WebService{
			Label:       ws.Label,
			Description: ws.Description,
			Url:         ws.Url,
			Status:      ws.Status,
		}
	}

	for iNs, ns := range st.Services.NameServer {
		ret.NameServers[iNs] = json.NameServer{
			Name:   ns.Name,
			Ping:   ns.Ping.PacketLoss <= 20,
			Lookup: ns.ResolveStatus,
		}
	}

	return ret
}

func (n *Node) IsOperational() bool {
	return n.ApiStatus && n.Ping.PacketLoss <= 20
}

func (r *DnsResolver) IsOperational() bool {
	return r.ResolveStatus
}

func (pc *PingCheck) IsUp() bool {
	return pc.PacketLoss <= 20
}
