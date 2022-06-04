package main

import (
	"time"

	"github.com/vpsfreecz/vpsf-status/config"
	"github.com/vpsfreecz/vpsf-status/json"
)

type Status struct {
	VpsAdmin      VpsAdmin
	LocationList  []*Location
	LocationMap   map[string]*Location
	GlobalNodeMap map[string]*Node
	Services      *Services
}

type VpsAdmin struct {
	Api     *WebService
	Webui   *WebService
	Console *WebService

	TotalUp    int
	TotalCount int
}

type Location struct {
	Id              int
	Label           string
	NodeList        []*Node
	NodeMap         map[string]*Node
	DnsResolverList []*DnsResolver

	TotalUp           int
	TotalCount        int
	NodesUp           int
	NodesCount        int
	DnsResolversUp    int
	DnsResolversCount int
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

	Up              int
	Count           int
	WebUp           int
	WebCount        int
	NameServerUp    int
	NameServerCount int
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

func (st *Status) CacheCounts() {
	st.VpsAdmin.CacheCounts()

	for _, loc := range st.LocationList {
		loc.CacheCounts()
	}

	st.Services.CacheCounts()
}

func (st *Status) ToJson(now time.Time, notice string) *json.Status {
	ret := &json.Status{
		VpsAdmin: json.VpsAdmin{
			Api:     st.VpsAdmin.Api.Status,
			Console: st.VpsAdmin.Console.Status,
			Webui:   st.VpsAdmin.Webui.Status,
		},
		Locations:   make([]json.Location, len(st.LocationList)),
		WebServices: make([]json.WebService, len(st.Services.Web)),
		NameServers: make([]json.NameServer, len(st.Services.NameServer)),
		Notice:      notice,
		GeneratedAt: now,
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
				Ping:        node.Ping.PacketLoss < 20,
				Maintenance: node.ApiMaintenance,
			}
		}

		for iDns, dns := range loc.DnsResolverList {
			jsonLoc.DnsResolvers[iDns] = json.DnsResolver{
				Name:   dns.Name,
				Ping:   dns.Ping.PacketLoss < 20,
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
			Ping:   ns.Ping.PacketLoss < 20,
			Lookup: ns.ResolveStatus,
		}
	}

	return ret
}

func (vpsa *VpsAdmin) IsOperational() bool {
	return vpsa.Api.Status && vpsa.Webui.Status && vpsa.Console.Status
}

func (vpsa *VpsAdmin) CacheCounts() {
	vpsa.TotalUp = vpsa.GetTotalUp()
	vpsa.TotalCount = 3
}

func (vpsa *VpsAdmin) GetTotalUp() int {
	cnt := 0

	for _, ws := range []*WebService{vpsa.Api, vpsa.Webui, vpsa.Console} {
		if ws.Status {
			cnt += 1
		}
	}

	return cnt
}

func (loc *Location) CacheCounts() {
	loc.TotalUp = loc.GetTotalUp()
	loc.TotalCount = loc.GetTotalCount()
	loc.NodesUp = loc.GetNodesUp()
	loc.NodesCount = loc.GetNodesCount()
	loc.DnsResolversUp = loc.GetDnsResolversUp()
	loc.DnsResolversCount = loc.GetDnsResolversCount()
}

func (loc *Location) IsOperational() bool {
	return loc.TotalUp == loc.TotalCount
}

func (loc *Location) GetTotalUp() int {
	return loc.GetNodesUp() + loc.GetDnsResolversUp()
}

func (loc *Location) GetTotalCount() int {
	return loc.GetNodesCount() + loc.GetDnsResolversCount()
}

func (loc *Location) EvenNodes() []*Node {
	return loc.SelectNodes(func(i int, n *Node) bool {
		return (i+1)%2 == 0
	})
}

func (loc *Location) OddNodes() []*Node {
	return loc.SelectNodes(func(i int, n *Node) bool {
		return (i+1)%2 == 1
	})
}

func (loc *Location) SelectNodes(filter func(int, *Node) bool) []*Node {
	result := make([]*Node, 0)

	for i, node := range loc.NodeList {
		if filter(i, node) {
			result = append(result, node)
		}
	}

	return result
}

func (loc *Location) GetNodesUp() int {
	cnt := 0

	for _, node := range loc.NodeList {
		if node.IsOperational() {
			cnt += 1
		}
	}

	return cnt
}

func (loc *Location) GetNodesCount() int {
	return len(loc.NodeList)
}

func (loc *Location) NodesOperational() bool {
	return loc.NodesUp == loc.NodesCount
}

func (loc *Location) HasDnsResolvers() bool {
	return len(loc.DnsResolverList) > 0
}

func (loc *Location) GetDnsResolversUp() int {
	cnt := 0

	for _, r := range loc.DnsResolverList {
		if r.IsOperational() {
			cnt += 1
		}
	}

	return cnt
}

func (loc *Location) GetDnsResolversCount() int {
	return len(loc.DnsResolverList)
}

func (loc *Location) DnsResolversOperational() bool {
	return loc.DnsResolversUp == loc.DnsResolversCount
}

func (n *Node) IsOperational() bool {
	return n.ApiStatus && n.Ping.PacketLoss < 20
}

func (r *DnsResolver) IsOperational() bool {
	return r.ResolveStatus
}

func (s *Services) CacheCounts() {
	cnt := 0
	for _, ws := range s.Web {
		if ws.Status {
			cnt += 1
		}
	}
	s.WebUp = cnt
	s.WebCount = len(s.Web)

	cnt = 0
	for _, ns := range s.NameServer {
		if ns.ResolveStatus {
			cnt += 1
		}
	}
	s.NameServerUp = cnt
	s.NameServerCount = len(s.NameServer)

	s.Up = s.WebUp + s.NameServerUp
	s.Count = s.WebCount + s.NameServerCount
}

func (s *Services) IsOperational() bool {
	return s.Up == s.Count
}

func (s *Services) IsWebOperational() bool {
	return s.WebUp == s.WebCount
}

func (s *Services) IsNameServerOperational() bool {
	return s.NameServerUp == s.NameServerCount
}

func (pc *PingCheck) IsUp() bool {
	return pc.PacketLoss < 20
}
