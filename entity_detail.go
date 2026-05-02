package main

import (
	"net/http"
	"strings"
	"time"
)

type EntityDetailView struct {
	Kind        string
	ID          string
	Label       string
	Group       string
	StatusText  string
	StatusClass string
	History     HistoryBarView
	Events      []ProbeEventView
}

type ProbeEventView struct {
	ChangedAt   string
	Method      string
	Status      string
	StatusClass string
	Message     string
}

func (e EntityDetailView) HasEvents() bool {
	return len(e.Events) > 0
}

func createEntityDetailView(st *Status, kind string, id string, now time.Time) (EntityDetailView, bool) {
	if st == nil || kind == "" || id == "" {
		return EntityDetailView{}, false
	}

	_, bars := createHistoryViews(st, now)
	history := bars[historyKey(kind, id)]

	ret := EntityDetailView{
		Kind:    kind,
		ID:      id,
		History: history,
	}

	switch kind {
	case historyEntityNode:
		node := st.GlobalNodeMap[id]
		if node == nil {
			return EntityDetailView{}, false
		}
		ret.Label = node.Name
		ret.Group = "Node"
		ret.StatusText, ret.StatusClass = nodeStatusText(node)
	case historyEntityVpsAdmin:
		ws := vpsAdminServiceByID(st, id)
		if ws == nil {
			return EntityDetailView{}, false
		}
		ret.Label = vpsAdminServiceLabel(id)
		ret.Group = "vpsAdmin"
		ret.StatusText, ret.StatusClass = webServiceStatusText(ws)
	case historyEntityDnsResolver:
		resolver := findDnsResolver(st, id)
		if resolver == nil {
			return EntityDetailView{}, false
		}
		ret.Label = resolver.Name
		ret.Group = "DNS Resolver"
		ret.StatusText, ret.StatusClass = dnsResolverStatusText(resolver)
	case historyEntityWebService:
		ws := findWebService(st, id)
		if ws == nil {
			return EntityDetailView{}, false
		}
		ret.Label = ws.Label
		ret.Group = "Web Service"
		ret.StatusText, ret.StatusClass = webServiceStatusText(ws)
	case historyEntityNameServer:
		resolver := findNameServer(st, id)
		if resolver == nil {
			return EntityDetailView{}, false
		}
		ret.Label = resolver.Name
		ret.Group = "Name Server"
		ret.StatusText, ret.StatusClass = dnsResolverStatusText(resolver)
	default:
		return EntityDetailView{}, false
	}

	if st.History != nil {
		for _, event := range st.History.ProbeEventsFor(kind, id, now, historyDaysForStatus(st)) {
			ret.Events = append(ret.Events, ProbeEventView{
				ChangedAt:   event.ChangedAt.Local().Format("2006-01-02 15:04 MST"),
				Method:      event.Method,
				Status:      statusTitle(event.Status),
				StatusClass: probeStatusClass(event.Status),
				Message:     event.Message,
			})
		}
	}

	return ret, true
}

func vpsAdminServiceByID(st *Status, id string) *WebService {
	switch id {
	case "api":
		return st.VpsAdmin.Api
	case "webui":
		return st.VpsAdmin.Webui
	case "console":
		return st.VpsAdmin.Console
	default:
		return nil
	}
}

func findDnsResolver(st *Status, id string) *DnsResolver {
	for _, loc := range st.LocationList {
		for _, resolver := range loc.DnsResolverList {
			if resolver.Name == id {
				return resolver
			}
		}
	}
	return nil
}

func findWebService(st *Status, id string) *WebService {
	for _, ws := range st.Services.Web {
		if ws.Label == id {
			return ws
		}
	}
	return nil
}

func findNameServer(st *Status, id string) *DnsResolver {
	for _, ns := range st.Services.NameServer {
		if ns.Name == id {
			return ns
		}
	}
	return nil
}

func nodeStatusText(node *Node) (string, string) {
	if node.IsOperational() {
		return "Operational", "success"
	}
	if node.IsDegraded() {
		return "Degraded", "warning"
	}
	return "Down", "danger"
}

func dnsResolverStatusText(resolver *DnsResolver) (string, string) {
	if resolver.IsOperational() {
		return "Operational", "success"
	}
	if resolver.IsDegraded() {
		return "Degraded", "warning"
	}
	return "Down", "danger"
}

func webServiceStatusText(ws *WebService) (string, string) {
	switch ws.StatusString() {
	case "operational":
		return "Operational", "success"
	case "maintenance":
		return "Under maintenance", "warning"
	default:
		if ws.StatusCode != 0 {
			return http.StatusText(ws.StatusCode), "danger"
		}
		return "Down", "danger"
	}
}

func probeStatusClass(status string) string {
	switch status {
	case historyProbeStateOperational:
		return "success"
	case historyProbeStateMaintenance, historyProbeStateDegraded:
		return "warning"
	default:
		return "danger"
	}
}

func statusTitle(status string) string {
	if status == "" {
		return "Unknown"
	}

	parts := strings.Split(status, "_")
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}

	return strings.Join(parts, " ")
}
