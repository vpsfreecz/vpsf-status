package main

type StatusView struct {
	VpsAdmin  VpsAdminView
	Locations []LocationView
	Services  ServicesView
}

type VpsAdminView struct {
	VpsAdmin

	TotalUp    int
	TotalCount int
}

type LocationView struct {
	Location

	TotalUp           int
	TotalCount        int
	NodesUp           int
	NodesCount        int
	DnsResolversUp    int
	DnsResolversCount int
}

type ServicesView struct {
	Services

	Up              int
	Count           int
	WebUp           int
	WebCount        int
	NameServerUp    int
	NameServerCount int
}

func createStatusView(st *Status) StatusView {
	return StatusView{
		VpsAdmin:  createVpsAdminView(st.VpsAdmin),
		Locations: createLocationView(st.LocationList),
		Services:  createServicesView(st.Services),
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

		v.TotalUp = v.getTotalUp()
		v.TotalCount = v.getTotalCount()
		v.NodesUp = v.getNodesUp()
		v.NodesCount = v.getNodesCount()
		v.DnsResolversUp = v.getDnsResolversUp()
		v.DnsResolversCount = v.getDnsResolversCount()

		ret[i] = v
	}

	return ret
}

func (loc *LocationView) IsOperational() bool {
	return loc.TotalUp == loc.TotalCount
}

func (loc *LocationView) getTotalUp() int {
	return loc.getNodesUp() + loc.getDnsResolversUp()
}

func (loc *LocationView) getTotalCount() int {
	return loc.getNodesCount() + loc.getDnsResolversCount()
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

func (loc *LocationView) getNodesUp() int {
	cnt := 0

	for _, node := range loc.NodeList {
		if node.IsOperational() {
			cnt += 1
		}
	}

	return cnt
}

func (loc *LocationView) getNodesCount() int {
	return len(loc.NodeList)
}

func (loc *LocationView) NodesOperational() bool {
	return loc.NodesUp == loc.NodesCount
}

func (loc *LocationView) HasDnsResolvers() bool {
	return len(loc.DnsResolverList) > 0
}

func (loc *LocationView) getDnsResolversUp() int {
	cnt := 0

	for _, r := range loc.DnsResolverList {
		if r.IsOperational() {
			cnt += 1
		}
	}

	return cnt
}

func (loc *LocationView) getDnsResolversCount() int {
	return len(loc.DnsResolverList)
}

func (loc *LocationView) DnsResolversOperational() bool {
	return loc.DnsResolversUp == loc.DnsResolversCount
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
		if ns.ResolveStatus {
			ret.NameServerUp += 1
		}
	}
	ret.NameServerCount = len(ret.NameServer)

	ret.Up = ret.WebUp + ret.NameServerUp
	ret.Count = ret.WebCount + ret.NameServerCount

	return ret
}

func (s *ServicesView) IsOperational() bool {
	return s.Up == s.Count
}

func (s *ServicesView) IsWebOperational() bool {
	return s.WebUp == s.WebCount
}

func (s *ServicesView) IsNameServerOperational() bool {
	return s.NameServerUp == s.NameServerCount
}
