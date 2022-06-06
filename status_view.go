package main

type StatusView struct {
	VpsAdmin      VpsAdminView
	Locations     []LocationView
	Services      ServicesView
	OutageReports *OutageReports
}

type VpsAdminView struct {
	VpsAdmin

	TotalUp    int
	TotalCount int
}

type LocationView struct {
	Location

	TotalUp              int
	TotalCount           int
	NodesUp              int
	NodesCount           int
	NodesDegraded        bool
	DnsResolversUp       int
	DnsResolversCount    int
	DnsResolversDegraded bool
	Degraded             bool
}

type ServicesView struct {
	Services

	Up                 int
	Count              int
	WebUp              int
	WebCount           int
	NameServerUp       int
	NameServerCount    int
	NameServerDegraded bool
	Degraded           bool
}

func createStatusView(st *Status) StatusView {
	return StatusView{
		VpsAdmin:      createVpsAdminView(st.VpsAdmin),
		Locations:     createLocationView(st.LocationList),
		Services:      createServicesView(st.Services),
		OutageReports: st.OutageReports,
	}
}

func createVpsAdminView(vpsa VpsAdmin) VpsAdminView {
	ret := VpsAdminView{
		VpsAdmin:   vpsa,
		TotalCount: 3,
	}

	ret.TotalUp = 0
	for _, ws := range []*WebService{vpsa.Api, vpsa.Webui, vpsa.Console} {
		if ws.Status {
			ret.TotalUp += 1
		}
	}

	return ret
}

func (vpsa *VpsAdminView) IsOperational() bool {
	return vpsa.Api.Status && vpsa.Webui.Status && vpsa.Console.Status
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
			if r.IsOperational() {
				v.DnsResolversUp += 1
			} else if r.IsDegraded() {
				v.DnsResolversUp += 1
				v.DnsResolversDegraded = true
			}
		}

		v.TotalUp = v.NodesUp + v.DnsResolversUp
		v.TotalCount = v.NodesCount + v.DnsResolversCount

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
		if ws.Status {
			ret.WebUp += 1
		}
	}
	ret.WebCount = len(ret.Web)

	ret.NameServerUp = 0
	for _, ns := range ret.NameServer {
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

	if ret.NameServerDegraded {
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
	return s.WebUp == s.WebCount
}

func (s *ServicesView) IsNameServerOperational() bool {
	return s.NameServerUp == s.NameServerCount && !s.NameServerDegraded
}

func (s *ServicesView) IsNameServerDegraded() bool {
	return s.NameServerUp == s.NameServerCount && s.NameServerDegraded
}
