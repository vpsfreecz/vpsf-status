package main

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/vpsfreecz/vpsf-status/config"
	"github.com/vpsfreecz/vpsf-status/json"
)

type Status struct {
	Initialized        bool
	VpsAdmin           VpsAdmin
	VpsAdminLocations  map[int64]VpsAdminLocation
	OutageReports      *OutageReports
	SecurityAdvisories *SecurityAdvisories
	History            *HistoryStore
	HistoryDays        int
	LocationList       []*Location
	LocationMap        map[string]*Location
	GlobalNodeMap      map[string]*Node
	Services           *Services
	Exporter           *Exporter

	indexHistoryVersion atomic.Uint64
	requestIndexRender  func()
}

type VpsAdmin struct {
	Api     *WebService
	Webui   *WebService
	Console *WebService
}

type VpsAdminLocation struct {
	Id               int64
	Label            string
	EnvironmentId    int64
	EnvironmentLabel string
}

type OutageReports struct {
	Status             bool
	ActiveList         []*OutageReport
	RecentList         []*OutageReport
	AnyActive          bool
	AnyActivePlanned   bool
	AnyActiveUnplanned bool
	AnyRecent          bool
	AnyRecentPlanned   bool
	AnyRecentUnplanned bool
}

type OutageReport struct {
	Id               int64
	BeginsAt         time.Time
	FinishedAt       time.Time
	Duration         time.Duration
	Type             string
	State            string
	Impact           string
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

type SecurityAdvisories struct {
	Status     bool
	RecentList []*SecurityAdvisory
	AnyRecent  bool
}

type SecurityAdvisory struct {
	Id                int64
	PublishedAt       time.Time
	UpdatedAt         time.Time
	State             string
	Cves              []SecurityAdvisoryCve
	Name              string
	EnSummary         string
	EnDescription     string
	EnResponse        string
	AffectedNodeCount int64
}

type SecurityAdvisoryCve struct {
	Id    int64
	CveId string
	Url   string
}

const (
	outageTypePlanned           = "planned_outage"
	outageTypeUnplanned         = "unplanned_outage"
	legacyOutageTypeMaintenance = "maintenance"
	legacyOutageTypeOutage      = "outage"
)

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
		VpsAdminLocations: make(map[int64]VpsAdminLocation),
		HistoryDays:       cfg.HistoryDays,
		LocationList:      make([]*Location, len(cfg.Locations)),
		LocationMap:       make(map[string]*Location),
		GlobalNodeMap:     make(map[string]*Node),
		Services: &Services{
			Web:        make([]*WebService, len(cfg.WebServices)),
			NameServer: make([]*DnsResolver, len(cfg.NameServers)),
		},
		OutageReports: &OutageReports{
			ActiveList: make([]*OutageReport, 0),
			RecentList: make([]*OutageReport, 0),
		},
		SecurityAdvisories: &SecurityAdvisories{
			RecentList: make([]*SecurityAdvisory, 0),
		},
		Exporter: newExporter(),
	}
	if st.HistoryDays == 0 {
		st.HistoryDays = config.DefaultHistoryDays
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

func recordConfiguredEntitySnapshots(st *Status, now time.Time) error {
	if st == nil || st.History == nil {
		return nil
	}

	snapshots := make([]HistoryEntitySnapshot, 0)
	for _, loc := range st.LocationList {
		for _, node := range loc.NodeList {
			locationID := int64(node.LocationId)
			if locationID == 0 {
				locationID = int64(loc.Id)
			}

			snapshots = append(snapshots, HistoryEntitySnapshot{
				EntityKind:         historyEntityNode,
				EntityID:           node.Name,
				EntityLabel:        node.Name,
				NodeID:             int64(node.Id),
				GroupKind:          historyGroupLocation,
				GroupID:            loc.Id,
				GroupLabel:         loc.Label,
				VpsAdminLocationID: locationID,
				LastSeen:           now,
			})
		}
	}

	return st.History.RecordEntitySnapshots(snapshots)
}

func (st *Status) initialize(cfg *config.Config) {
	checkInterval := time.Duration(cfg.CheckInterval) * time.Second
	checkTimeout := time.Duration(cfg.CheckTimeout) * time.Second

	time.Sleep(5 * time.Second)

	go checkNoticeFile(st, cfg.NoticeFile, checkInterval)

	go checkApi(st, checkInterval, checkTimeout)
	time.Sleep(1 * time.Second)

	go checkOutageReports(st, checkInterval, checkTimeout)
	time.Sleep(1 * time.Second)

	go checkSecurityAdvisories(st, checkInterval, checkTimeout)
	time.Sleep(1 * time.Second)

	checkVpsAdminWebServices(st, checkInterval, checkTimeout)
	time.Sleep(1 * time.Second)

	pingNodes(st, checkInterval)
	time.Sleep(1 * time.Second)

	pingDnsResolvers(st, checkInterval)
	time.Sleep(1 * time.Second)

	checkDnsResolvers(st, checkInterval, checkTimeout)
	time.Sleep(1 * time.Second)

	checkWebServices(st, checkInterval, checkTimeout)
	time.Sleep(1 * time.Second)

	pingNameServers(st, checkInterval)
	time.Sleep(1 * time.Second)

	checkNameServers(st, checkInterval, checkTimeout)

	time.Sleep(5 * time.Second)
	st.Initialized = true
	st.Exporter.up.Set(1)
	st.requestIndexRenderIfConfigured()
}

func (st *Status) requestIndexRenderIfConfigured() {
	if st == nil || st.requestIndexRender == nil {
		return
	}

	st.requestIndexRender()
}

func (st *Status) markIndexHistoryChanged() {
	if st == nil {
		return
	}

	st.indexHistoryVersion.Add(1)
	st.requestIndexRenderIfConfigured()
}

func (st *Status) ToJson(now time.Time, notice Notice) *json.Status {
	outages := st.OutageReports
	advisories := st.SecurityAdvisories

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
		SecurityAdvisories: json.SecurityAdvisories{
			Status: advisories.Status,
			Recent: make(
				[]json.SecurityAdvisory,
				len(advisories.RecentList),
			),
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
			Type:          outage.NormalizedType(),
			State:         outage.State,
			Impact:        outage.Impact,
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
			Type:          outage.NormalizedType(),
			State:         outage.State,
			Impact:        outage.Impact,
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

	for iAdvisory, advisory := range advisories.RecentList {
		cves := make([]json.SecurityAdvisoryCve, len(advisory.Cves))
		for iCve, cve := range advisory.Cves {
			cves[iCve] = json.SecurityAdvisoryCve{
				Id:    cve.Id,
				CveId: cve.CveId,
				Url:   cve.Url,
			}
		}

		ret.SecurityAdvisories.Recent[iAdvisory] = json.SecurityAdvisory{
			Id:                advisory.Id,
			PublishedAt:       advisory.PublishedAt,
			UpdatedAt:         advisory.UpdatedAt,
			State:             advisory.State,
			Cves:              cves,
			Name:              advisory.Name,
			EnSummary:         advisory.EnSummary,
			EnDescription:     advisory.EnDescription,
			EnResponse:        advisory.EnResponse,
			AffectedNodeCount: advisory.AffectedNodeCount,
		}
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

func (r *OutageReport) IsPlannedOutage() bool {
	return r.NormalizedType() == outageTypePlanned
}

func (r *OutageReport) IsUnplannedOutage() bool {
	return r.NormalizedType() == outageTypeUnplanned
}

func (r *OutageReport) NormalizedType() string {
	if r == nil {
		return ""
	}
	return normalizeOutageType(r.Type)
}

func (r *OutageReport) TypeLabel() string {
	switch r.NormalizedType() {
	case outageTypePlanned:
		return "Planned outage"
	case outageTypeUnplanned:
		return "Unplanned outage"
	default:
		return r.Type
	}
}

func (r *OutageReport) ImpactLabel() string {
	if r == nil {
		return ""
	}

	switch r.Impact {
	case "tbd":
		return "TBD"
	case "system_restart":
		return "System restart"
	case "system_reset":
		return "System reset"
	case "network":
		return "Network"
	case "performance":
		return "Performance"
	case "unavailability":
		return "Unavailability"
	case "export":
		return "NFS export"
	default:
		return r.Impact
	}
}

func normalizeOutageType(outageType string) string {
	switch outageType {
	case legacyOutageTypeMaintenance:
		return outageTypePlanned
	case legacyOutageTypeOutage:
		return outageTypeUnplanned
	default:
		return outageType
	}
}

func (n *Node) IsOperational() bool {
	return n.Ping.IsUp() && n.ApiStatus && n.IsStorageOperational()
}

func (n *Node) IsDegraded() bool {
	if !n.Ping.IsUp() && !n.Ping.IsWarning() {
		return false
	}

	if n.IsStorageHardFailure() {
		return false
	}

	return !n.IsOperational()
}

func (n *Node) IsStorageSupported() bool {
	return n.OsType == "vpsadminos"
}

func (n *Node) IsStorageOperational() bool {
	if !n.IsStorageSupported() {
		return true
	}

	scanOperational := n.PoolScan == "none" || n.IsStorageScrubIssue()
	return n.PoolStatus && n.PoolState == "online" && scanOperational
}

func (n *Node) IsStorageDegraded() bool {
	if !n.IsStorageSupported() {
		return false
	}

	scanDegraded := n.PoolScan == "resilver"
	return n.PoolStatus && (n.PoolState == "degraded" || scanDegraded)
}

func (n *Node) IsStorageHardFailure() bool {
	if !n.IsStorageSupported() {
		return false
	}

	return n.PoolStatus && (n.PoolState == "suspended" || n.PoolState == "faulted")
}

func (n *Node) IsStorageStateIssue() bool {
	return n.IsStorageSupported() && n.PoolStatus && n.PoolState != "online"
}

func (n *Node) IsStorageScanIssue() bool {
	if !n.IsStorageSupported() || !n.PoolStatus {
		return false
	}

	return n.PoolScan != "none" && !n.IsStorageScrubIssue() && !n.IsStorageResilverIssue()
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
	case "unknown":
		return "Unable to determine storage status"
	case "error":
		return "Storage status check failed"
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
	case "unknown":
		return "Unable to determine storage scan status"
	case "error":
		return "Storage scan status check failed"
	default:
		return "Storage scan is in a unknown state"
	}
}

func (r *DnsResolver) IsOperational() bool {
	return r.ResolveStatus && r.Ping.IsUp()
}

func (r *DnsResolver) IsDegraded() bool {
	return r.ResolveStatus && r.Ping.IsWarning()
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
