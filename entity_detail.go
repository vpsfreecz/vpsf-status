package main

import (
	"net/http"
	"strings"
	"time"
)

type EntityDetailView struct {
	Kind                     string
	ID                       string
	Label                    string
	Group                    string
	StatusText               string
	StatusClass              string
	History                  HistoryBarView
	Availability             []AvailabilityView
	Events                   []ProbeEventView
	ShowReportedAvailability bool
	ShowEventEntity          bool
}

type AvailabilityView struct {
	Label             string
	Reported          string
	ReportedAvailable bool
	Probe             string
	ProbeAvailable    bool
}

type ProbeEventView struct {
	ChangedAt   string
	Entity      string
	Method      string
	Status      string
	StatusClass string
	Message     string
}

func (e EntityDetailView) HasEvents() bool {
	return len(e.Events) > 0
}

func (e EntityDetailView) HasAvailability() bool {
	return len(e.Availability) > 0
}

func createEntityDetailView(st *Status, kind string, id string, now time.Time) (EntityDetailView, bool) {
	if st == nil || kind == "" || id == "" {
		return EntityDetailView{}, false
	}

	ret := EntityDetailView{
		Kind:                     kind,
		ID:                       id,
		ShowReportedAvailability: availabilityReportedOutageSupported(kind),
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

	data := newHistoryData(st, now)
	ret.History = createEntityHistoryView(st, now, kind, id, ret.Label, data)
	ret.Availability = availabilityDetailViews(entityAvailabilityWithData(st, kind, id, now, data))

	if st.History != nil {
		ret.Events = probeEventDetailViews(st.History.ProbeEventsFor(kind, id, now, historyDaysForStatus(st)))
	}

	return ret, true
}

func availabilityDetailViews(stats []availabilityResult) []AvailabilityView {
	ret := make([]AvailabilityView, 0, len(stats))
	for _, stat := range stats {
		view := AvailabilityView{
			Label:    stat.Label,
			Reported: "n/a",
			Probe:    "n/a",
		}
		if stat.Reported.Available {
			view.ReportedAvailable = true
			view.Reported = formatAvailabilityPercent(stat.Reported.Percent)
		}
		if stat.Probe.Available {
			view.ProbeAvailable = true
			view.Probe = formatAvailabilityPercent(stat.Probe.Percent)
		}
		ret = append(ret, view)
	}
	return ret
}

func probeEventDetailViews(events []ProbeEvent) []ProbeEventView {
	ret := make([]ProbeEventView, 0, len(events))
	for _, event := range events {
		ret = append(ret, ProbeEventView{
			ChangedAt:   event.ChangedAt.Local().Format("2006-01-02 15:04 MST"),
			Entity:      probeEventEntityLabel(event),
			Method:      event.Method,
			Status:      statusTitle(event.Status),
			StatusClass: probeStatusClass(event.Status),
			Message:     event.Message,
		})
	}
	return ret
}

func probeEventEntityLabel(event ProbeEvent) string {
	if event.EntityLabel != "" {
		return event.EntityLabel
	}
	return event.EntityID
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
