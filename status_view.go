package main

import "time"

type StatusView struct {
	VpsAdmin      VpsAdminView
	Locations     []LocationView
	Services      ServicesView
	OutageReports *OutageReports
	History       map[string]HistoryBarView
}

type VpsAdminView struct {
	VpsAdmin

	History    HistoryBarView
	Counts     StatusCounts
	TotalUp    int
	TotalCount int
}

type StatusCounts struct {
	Operational int
	Degraded    int
	Down        int
	Total       int
}

type LocationView struct {
	Location

	Counts               StatusCounts
	NodeCounts           StatusCounts
	DnsResolverCounts    StatusCounts
	TotalUp              int
	TotalCount           int
	NodesUp              int
	NodesCount           int
	NodesDegraded        bool
	DnsResolversUp       int
	DnsResolversCount    int
	DnsResolversDegraded bool
	Degraded             bool
	History              HistoryBarView
}

type ServicesView struct {
	Services

	Counts             StatusCounts
	WebCounts          StatusCounts
	NameServerCounts   StatusCounts
	Up                 int
	Count              int
	WebUp              int
	WebCount           int
	WebDegraded        bool
	NameServerUp       int
	NameServerCount    int
	NameServerDegraded bool
	Degraded           bool
	History            HistoryBarView
}

func createStatusView(st *Status, now time.Time) StatusView {
	ret := StatusView{
		VpsAdmin:      createVpsAdminView(st.VpsAdmin),
		Locations:     createLocationView(st.LocationList),
		Services:      createServicesView(st.Services),
		OutageReports: st.OutageReports,
	}

	groups, history := createHistoryViews(st, now)
	ret.History = history
	ret.VpsAdmin.History = groups.VpsAdmin
	for i := range ret.Locations {
		ret.Locations[i].History = groups.Locations[ret.Locations[i].Id]
	}
	ret.Services.History = groups.Services

	return ret
}

func createVpsAdminView(vpsa VpsAdmin) VpsAdminView {
	ret := VpsAdminView{
		VpsAdmin:   vpsa,
		TotalCount: 3,
	}

	for _, ws := range []*WebService{vpsa.Api, vpsa.Webui, vpsa.Console} {
		ret.Counts = ret.Counts.Add(webServiceCounts(ws))
	}
	ret.TotalUp = ret.Counts.Operational + ret.Counts.Degraded
	ret.TotalCount = ret.Counts.Total

	return ret
}

func (vpsa *VpsAdminView) IsOperational() bool {
	return vpsa.Api.Status && vpsa.Webui.Status && vpsa.Console.Status
}

func (vpsa *VpsAdminView) IsDegraded() bool {
	degraded := false

	for _, ws := range []*WebService{vpsa.Api, vpsa.Webui, vpsa.Console} {
		if !ws.Status && !ws.Maintenance {
			return false
		}

		if ws.Maintenance {
			degraded = true
		}
	}

	return degraded
}

func createLocationView(locations []*Location) []LocationView {
	ret := make([]LocationView, len(locations))

	for i, loc := range locations {
		v := LocationView{
			Location: *loc,
		}

		v.NodesUp = 0
		v.NodesCount = len(loc.NodeList)

		for _, node := range loc.NodeList {
			v.NodeCounts = v.NodeCounts.Add(nodeCounts(node))
			if node.IsOperational() {
				v.NodesUp += 1
			} else if node.IsDegraded() {
				v.NodesUp += 1
				v.NodesDegraded = true
			}
		}

		v.DnsResolversUp = 0
		v.DnsResolversCount = len(loc.DnsResolverList)

		for _, r := range loc.DnsResolverList {
			v.DnsResolverCounts = v.DnsResolverCounts.Add(dnsResolverCounts(r))
			if r.IsOperational() {
				v.DnsResolversUp += 1
			} else if r.IsDegraded() {
				v.DnsResolversUp += 1
				v.DnsResolversDegraded = true
			}
		}

		v.TotalUp = v.NodesUp + v.DnsResolversUp
		v.TotalCount = v.NodesCount + v.DnsResolversCount
		v.Counts = v.NodeCounts.Add(v.DnsResolverCounts)

		if v.NodesDegraded || v.DnsResolversDegraded {
			v.Degraded = true
		}

		ret[i] = v
	}

	return ret
}

func (loc *LocationView) IsOperational() bool {
	return loc.TotalUp == loc.TotalCount && !loc.Degraded
}

func (loc *LocationView) IsDegraded() bool {
	return loc.TotalUp == loc.TotalCount && loc.Degraded
}

func (loc *LocationView) EvenNodes() []*Node {
	return loc.SelectNodes(func(i int, n *Node) bool {
		return (i+1)%2 == 0
	})
}

func (loc *LocationView) OddNodes() []*Node {
	return loc.SelectNodes(func(i int, n *Node) bool {
		return (i+1)%2 == 1
	})
}

func (loc *LocationView) SelectNodes(filter func(int, *Node) bool) []*Node {
	result := make([]*Node, 0)

	for i, node := range loc.NodeList {
		if filter(i, node) {
			result = append(result, node)
		}
	}

	return result
}

func (loc *LocationView) AreNodesOperational() bool {
	return loc.NodesUp == loc.NodesCount && !loc.NodesDegraded
}

func (loc *LocationView) AreNodesDegraded() bool {
	return loc.NodesUp == loc.NodesCount && loc.NodesDegraded
}

func (loc *LocationView) HasDnsResolvers() bool {
	return len(loc.DnsResolverList) > 0
}

func (loc *LocationView) AreDnsResolversOperational() bool {
	return loc.DnsResolversUp == loc.DnsResolversCount && !loc.DnsResolversDegraded
}

func (loc *LocationView) AreDnsResolversDegraded() bool {
	return loc.DnsResolversUp == loc.DnsResolversCount && loc.DnsResolversDegraded
}

func createServicesView(services *Services) ServicesView {
	ret := ServicesView{
		Services: *services,
	}

	ret.WebUp = 0
	for _, ws := range ret.Web {
		ret.WebCounts = ret.WebCounts.Add(webServiceCounts(ws))
		if ws.Status {
			ret.WebUp += 1
		} else if ws.Maintenance {
			ret.WebUp += 1
			ret.WebDegraded = true
		}
	}
	ret.WebCount = len(ret.Web)

	ret.NameServerUp = 0
	for _, ns := range ret.NameServer {
		ret.NameServerCounts = ret.NameServerCounts.Add(dnsResolverCounts(ns))
		if ns.IsOperational() {
			ret.NameServerUp += 1
		} else if ns.IsDegraded() {
			ret.NameServerUp += 1
			ret.NameServerDegraded = true
		}
	}
	ret.NameServerCount = len(ret.NameServer)

	ret.Up = ret.WebUp + ret.NameServerUp
	ret.Count = ret.WebCount + ret.NameServerCount
	ret.Counts = ret.WebCounts.Add(ret.NameServerCounts)

	if ret.WebDegraded || ret.NameServerDegraded {
		ret.Degraded = true
	}

	return ret
}

func (s *ServicesView) IsOperational() bool {
	return s.Up == s.Count && !s.Degraded
}

func (s *ServicesView) IsDegraded() bool {
	return s.Up == s.Count && s.Degraded
}

func (s *ServicesView) IsWebOperational() bool {
	return s.WebUp == s.WebCount && !s.WebDegraded
}

func (s *ServicesView) IsWebDegraded() bool {
	return s.WebUp == s.WebCount && s.WebDegraded
}

func (s *ServicesView) IsNameServerOperational() bool {
	return s.NameServerUp == s.NameServerCount && !s.NameServerDegraded
}

func (s *ServicesView) IsNameServerDegraded() bool {
	return s.NameServerUp == s.NameServerCount && s.NameServerDegraded
}

func (c StatusCounts) Add(other StatusCounts) StatusCounts {
	return StatusCounts{
		Operational: c.Operational + other.Operational,
		Degraded:    c.Degraded + other.Degraded,
		Down:        c.Down + other.Down,
		Total:       c.Total + other.Total,
	}
}

func nodeCounts(node *Node) StatusCounts {
	ret := StatusCounts{Total: 1}
	if node == nil {
		ret.Down = 1
	} else if node.IsOperational() {
		ret.Operational = 1
	} else if node.IsDegraded() {
		ret.Degraded = 1
	} else {
		ret.Down = 1
	}
	return ret
}

func dnsResolverCounts(resolver *DnsResolver) StatusCounts {
	ret := StatusCounts{Total: 1}
	if resolver == nil {
		ret.Down = 1
	} else if resolver.IsOperational() {
		ret.Operational = 1
	} else if resolver.IsDegraded() {
		ret.Degraded = 1
	} else {
		ret.Down = 1
	}
	return ret
}

func webServiceCounts(ws *WebService) StatusCounts {
	ret := StatusCounts{Total: 1}
	if ws != nil && ws.Status {
		ret.Operational = 1
	} else if ws != nil && ws.Maintenance {
		ret.Degraded = 1
	} else {
		ret.Down = 1
	}
	return ret
}
