package main

import (
	"fmt"
	"time"

	"github.com/vpsfreecz/vpsf-status/config"
	"github.com/vpsfreecz/vpsf-status/json"
)

type Status struct {
	Initialized   bool
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
	Status               bool
	ActiveList           []*OutageReport
	RecentList           []*OutageReport
	AnyActive            bool
	AnyActiveMaintenance bool
	AnyActiveOutage      bool
	AnyRecent            bool
	AnyRecentMaintenance bool
	AnyRecentOutage      bool
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
	OsType     string

	ApiStatus      bool
	ApiMaintenance bool
	LastApiCheck   time.Time

	PoolState       string
	PoolScan        string
	PoolScanPercent float64
	PoolStatus      bool

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
	Label       string
	Description string
	Url         string
	CheckUrl    string
	Method      string
	Status      bool
	Maintenance bool
	StatusCode  int
	LastCheck   time.Time
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
			ActiveList: make([]*OutageReport, 0),
			RecentList: make([]*OutageReport, 0),
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

func (st *Status) initialize(cfg *config.Config) {
	checkInterval := time.Duration(cfg.CheckInterval) * time.Second

	time.Sleep(5 * time.Second)

	go checkApi(st, checkInterval)
	time.Sleep(1 * time.Second)

	go checkOutageReports(st, checkInterval)
	time.Sleep(1 * time.Second)

	checkVpsAdminWebServices(st, checkInterval)
	time.Sleep(1 * time.Second)

	pingNodes(st, checkInterval)
	time.Sleep(1 * time.Second)

	pingDnsResolvers(st, checkInterval)
	time.Sleep(1 * time.Second)

	checkDnsResolvers(st, checkInterval)
	time.Sleep(1 * time.Second)

	checkWebServices(st, checkInterval)
	time.Sleep(1 * time.Second)

	checkNameServers(st, checkInterval)

	time.Sleep(5 * time.Second)
	st.Initialized = true
}

func (st *Status) ToJson(now time.Time, notice Notice) *json.Status {
	outages := st.OutageReports

	ret := &json.Status{
		VpsAdmin: json.VpsAdmin{
			Api: json.WebService{
				Label:       st.VpsAdmin.Api.Label,
				Description: st.VpsAdmin.Api.Description,
				Url:         st.VpsAdmin.Api.Url,
				Status:      st.VpsAdmin.Api.StatusString(),
			},
			Console: json.WebService{
				Label:       st.VpsAdmin.Console.Label,
				Description: st.VpsAdmin.Console.Description,
				Url:         st.VpsAdmin.Console.Url,
				Status:      st.VpsAdmin.Console.StatusString(),
			},
			Webui: json.WebService{
				Label:       st.VpsAdmin.Webui.Label,
				Description: st.VpsAdmin.Webui.Description,
				Url:         st.VpsAdmin.Webui.Url,
				Status:      st.VpsAdmin.Webui.StatusString(),
			},
		},
		OutageReports: json.OutageReports{
			Status:    outages.Status,
			Announced: make([]json.OutageReport, len(outages.ActiveList)),
			Recent:    make([]json.OutageReport, len(outages.RecentList)),
		},
		Locations:   make([]json.Location, len(st.LocationList)),
		WebServices: make([]json.WebService, len(st.Services.Web)),
		NameServers: make([]json.NameServer, len(st.Services.NameServer)),
		Notice: json.Notice{
			Any:       notice.Any,
			Text:      string(notice.Html),
			UpdatedAt: notice.UpdatedAt,
		},
		GeneratedAt: now,
	}

	for iOutage, outage := range outages.ActiveList {
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

	for iOutage, outage := range outages.RecentList {
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

		ret.OutageReports.Recent[iOutage] = jsonOutage
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
				Id:              node.Id,
				Name:            node.Name,
				LocationId:      node.LocationId,
				OsType:          node.OsType,
				VpsAdmin:        node.ApiStatus,
				Ping:            node.Ping.StatusString(),
				Maintenance:     node.ApiMaintenance,
				PoolState:       node.PoolState,
				PoolScan:        node.PoolScan,
				PoolScanPercent: node.PoolScanPercent,
				PoolStatus:      node.PoolStatus,
			}
		}

		for iDns, dns := range loc.DnsResolverList {
			jsonLoc.DnsResolvers[iDns] = json.DnsResolver{
				Name:   dns.Name,
				Ping:   dns.Ping.StatusString(),
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
			Status:      ws.StatusString(),
		}
	}

	for iNs, ns := range st.Services.NameServer {
		ret.NameServers[iNs] = json.NameServer{
			Name:   ns.Name,
			Ping:   ns.Ping.StatusString(),
			Lookup: ns.ResolveStatus,
		}
	}

	return ret
}

func (n *Node) IsOperational() bool {
	return n.ApiStatus && n.IsStorageOperational() && n.Ping.PacketLoss <= 20
}

func (n *Node) IsDegraded() bool {
	return n.ApiStatus && n.PoolStatus && (n.IsStorageDegraded() || (n.Ping.PacketLoss > 20 && n.Ping.PacketLoss < 100))
}

func (n *Node) IsStorageSupported() bool {
	return n.OsType == "vpsadminos"
}

func (n *Node) IsStorageOperational() bool {
	if !n.IsStorageSupported() {
		return true
	}

	return n.ApiStatus && n.PoolStatus && n.PoolState == "online" && n.PoolScan == "none"
}

func (n *Node) IsStorageDegraded() bool {
	if !n.IsStorageSupported() {
		return false
	}

	return n.ApiStatus && n.PoolStatus && (n.PoolState != "online" || n.PoolScan != "none")
}

func (n *Node) IsStorageStateIssue() bool {
	return n.PoolState != "online"
}

func (n *Node) IsStorageScrubIssue() bool {
	return n.PoolScan == "scrub"
}

func (n *Node) IsStorageResilverIssue() bool {
	return n.PoolScan == "resilver"
}

func (n *Node) GetStorageStateMessage() string {
	if !n.PoolStatus {
		return "Unable to determine storage status"
	}

	switch n.PoolState {
	case "online":
		return "Storage is online"
	case "degraded":
		return "One or more disks have failed, storage continues to function"
	case "suspended":
		return "Storage not operational"
	case "faulted":
		return "Storage not operational"
	default:
		return "Storage is in a unknown state"
	}
}

func (n *Node) GetStorageScanMessage() string {
	if !n.PoolStatus {
		return "Unable to determine storage status"
	}

	switch n.PoolScan {
	case "none":
		return "Not running"
	case "scrub":
		return fmt.Sprintf("Storage is being scrubbed to check data integrity, %.1f %% done", n.PoolScanPercent)
	case "resilver":
		return fmt.Sprintf("Storage is being resilvered to replace disks, %.1f %% done", n.PoolScanPercent)
	default:
		return "Storage scan is in a unknown state"
	}
}

func (r *DnsResolver) IsOperational() bool {
	return r.ResolveStatus
}

func (r *DnsResolver) IsDegraded() bool {
	return r.ResolveStatus && r.Ping.PacketLoss > 20 && r.Ping.PacketLoss < 100
}

func (ws *WebService) StatusString() string {
	if ws.Status {
		return "operational"
	} else if ws.Maintenance {
		return "maintenance"
	} else {
		return "down"
	}
}

func (pc *PingCheck) IsUp() bool {
	return pc.PacketLoss <= 20
}

func (pc *PingCheck) IsWarning() bool {
	return pc.PacketLoss > 20 && pc.PacketLoss < 100
}

func (pc *PingCheck) StatusString() string {
	if pc.IsUp() {
		return "responding"
	} else if pc.IsWarning() {
		return "degraded"
	} else {
		return "down"
	}
}

func (pc *PingCheck) IsDown() bool {
	return pc.PacketLoss == 100
}
