package main

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	probeLogPageParam = "probe_page"
	probeLogPageSize  = 50
)

type EntityDetailView struct {
	Lang                     string
	Kind                     string
	ID                       string
	Label                    string
	Group                    string
	StatusText               string
	StatusClass              string
	History                  HistoryBarView
	Availability             []AvailabilityView
	Events                   []ProbeEventView
	EventPagination          ProbeLogPaginationView
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
	CoveredBy   ProbeEventCoverageView
	GroupStart  bool
	GroupEnd    bool
}

type ProbeEventCoverageView struct {
	ID    int64
	Label string
	URL   string
	Class string
}

type ProbeLogPaginationView struct {
	Page        int
	Total       int
	TotalPages  int
	PreviousURL string
	NextURL     string
	Links       []ProbeLogPageLinkView
}

type ProbeLogPageLinkView struct {
	Label    string
	URL      string
	Current  bool
	Disabled bool
}

func (e EntityDetailView) HasEvents() bool {
	return len(e.Events) > 0
}

func (e EntityDetailView) HasAvailability() bool {
	return len(e.Availability) > 0
}

func (e ProbeEventView) HasCoverage() bool {
	return e.CoveredBy.ID != 0
}

func (p ProbeLogPaginationView) HasPages() bool {
	return p.TotalPages > 1
}

func (p ProbeLogPaginationView) HasPrevious() bool {
	return p.PreviousURL != ""
}

func (p ProbeLogPaginationView) HasNext() bool {
	return p.NextURL != ""
}

func createEntityDetailView(st *Status, kind string, id string, now time.Time, probePage int) (EntityDetailView, bool) {
	return createEntityDetailViewForLocale(st, kind, id, now, probePage, defaultPageLocale())
}

func createEntityDetailViewForLocale(st *Status, kind string, id string, now time.Time, probePage int, loc *pageLocale) (EntityDetailView, bool) {
	if st == nil || kind == "" || id == "" {
		return EntityDetailView{}, false
	}

	ret := EntityDetailView{
		Lang:                     loc.codeOrDefault(),
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
		ret.Group = loc.T("entity.group.node")
		ret.StatusText, ret.StatusClass = nodeStatusText(node, loc)
	case historyEntityVpsAdmin:
		ws := vpsAdminServiceByID(st, id)
		if ws == nil {
			return EntityDetailView{}, false
		}
		ret.Label = vpsAdminServiceLabelForLocale(id, loc)
		ret.Group = loc.T("entity.group.vpsadmin")
		ret.StatusText, ret.StatusClass = webServiceStatusText(ws, loc)
	case historyEntityDnsResolver:
		resolver := findDnsResolver(st, id)
		if resolver == nil {
			return EntityDetailView{}, false
		}
		ret.Label = resolver.Name
		ret.Group = loc.T("entity.group.dns_resolver")
		ret.StatusText, ret.StatusClass = dnsResolverStatusText(resolver, loc)
	case historyEntityWebService:
		ws := findWebService(st, id)
		if ws == nil {
			return EntityDetailView{}, false
		}
		ret.Label = ws.Label
		ret.Group = loc.T("entity.group.web_service")
		ret.StatusText, ret.StatusClass = webServiceStatusText(ws, loc)
	case historyEntityNameServer:
		resolver := findNameServer(st, id)
		if resolver == nil {
			return EntityDetailView{}, false
		}
		ret.Label = resolver.Name
		ret.Group = loc.T("entity.group.name_server")
		ret.StatusText, ret.StatusClass = dnsResolverStatusText(resolver, loc)
	default:
		return EntityDetailView{}, false
	}

	data := newHistoryData(st, now)
	ret.History = createEntityHistoryViewForLocale(st, now, kind, id, ret.Label, data, loc)
	ret.Availability = availabilityDetailViews(entityAvailabilityWithDataForLocale(st, kind, id, now, data, loc), loc)

	if st.History != nil {
		logPage, page := paginatedProbeLog(probePage, func(limit int, offset int) ProbeLogPage {
			return st.History.ProbeLogFor(kind, id, now, historyDaysForStatus(st), limit, offset)
		})
		ret.Events = probeLogEventDetailViewsForLocale(st, logPage.Events, now, data, loc)
		ret.EventPagination = newProbeLogPaginationView("/entity", kind, id, page, logPage.Total, loc.codeOrDefault())
	}

	return ret, true
}

func paginatedProbeLog(requestedPage int, query func(limit int, offset int) ProbeLogPage) (ProbeLogPage, int) {
	page := normalizeProbeLogPage(requestedPage)
	ret := query(probeLogPageSize, probeLogOffset(page))
	totalPages := probeLogTotalPages(ret.Total)
	if totalPages > 0 && page > totalPages {
		page = totalPages
		ret = query(probeLogPageSize, probeLogOffset(page))
	}
	return ret, page
}

func normalizeProbeLogPage(page int) int {
	if page < 1 {
		return 1
	}
	return page
}

func probeLogOffset(page int) int {
	return (normalizeProbeLogPage(page) - 1) * probeLogPageSize
}

func probeLogTotalPages(total int) int {
	if total <= 0 {
		return 0
	}
	return (total + probeLogPageSize - 1) / probeLogPageSize
}

func newProbeLogPaginationView(path string, kind string, id string, page int, total int, lang string) ProbeLogPaginationView {
	totalPages := probeLogTotalPages(total)
	page = normalizeProbeLogPage(page)
	if totalPages > 0 && page > totalPages {
		page = totalPages
	}

	ret := ProbeLogPaginationView{
		Page:       page,
		Total:      total,
		TotalPages: totalPages,
	}
	if totalPages <= 1 {
		return ret
	}

	if page > 1 {
		ret.PreviousURL = probeLogPageURL(path, kind, id, page-1, lang)
	}
	if page < totalPages {
		ret.NextURL = probeLogPageURL(path, kind, id, page+1, lang)
	}
	ret.Links = probeLogPageLinks(path, kind, id, page, totalPages, lang)
	return ret
}

func probeLogPageLinks(path string, kind string, id string, page int, totalPages int, lang string) []ProbeLogPageLinkView {
	if totalPages <= 1 {
		return nil
	}

	ret := make([]ProbeLogPageLinkView, 0)
	lastAdded := 0
	addPage := func(n int) {
		if n < 1 || n > totalPages || n <= lastAdded {
			return
		}
		if lastAdded != 0 && n > lastAdded+1 {
			ret = append(ret, ProbeLogPageLinkView{Label: "...", Disabled: true})
		}
		ret = append(ret, ProbeLogPageLinkView{
			Label:   strconv.Itoa(n),
			URL:     probeLogPageURL(path, kind, id, n, lang),
			Current: n == page,
		})
		lastAdded = n
	}

	addPage(1)
	for n := page - 2; n <= page+2; n++ {
		if n > 1 && n < totalPages {
			addPage(n)
		}
	}
	addPage(totalPages)

	return ret
}

func probeLogPageURL(path string, kind string, id string, page int, lang string) string {
	q := url.Values{}
	q.Set("kind", kind)
	if id != "" {
		q.Set("id", id)
	}
	if page > 1 {
		q.Set(probeLogPageParam, strconv.Itoa(page))
	}
	return localizedURL(path, lang, q)
}

func availabilityDetailViews(stats []availabilityResult, loc *pageLocale) []AvailabilityView {
	ret := make([]AvailabilityView, 0, len(stats))
	for _, stat := range stats {
		view := AvailabilityView{
			Label:    stat.Label,
			Reported: loc.T("status.not_available"),
			Probe:    loc.T("status.not_available"),
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

func probeLogEventDetailViews(st *Status, events []ProbeLogEvent, now time.Time, data *historyData) []ProbeEventView {
	return probeLogEventDetailViewsForLocale(st, events, now, data, defaultPageLocale())
}

func probeLogEventDetailViewsForLocale(st *Status, events []ProbeLogEvent, now time.Time, data *historyData, loc *pageLocale) []ProbeEventView {
	data = ensureHistoryData(st, now, data)

	ret := make([]ProbeEventView, 0, len(events))
	for _, event := range events {
		view := probeEventDetailViewForLocale(event.ProbeEvent, loc)
		if report := probeEventResponsibleReport(event, data.reports, data.mapping, now); report != nil {
			view.CoveredBy = probeEventCoverageView(st, report, loc)
		}
		ret = append(ret, view)
	}

	setProbeEventCoverageGroups(ret)
	return ret
}

func probeEventDetailViews(events []ProbeEvent) []ProbeEventView {
	ret := make([]ProbeEventView, 0, len(events))
	for _, event := range events {
		ret = append(ret, probeEventDetailView(event))
	}
	return ret
}

func probeEventDetailView(event ProbeEvent) ProbeEventView {
	return probeEventDetailViewForLocale(event, defaultPageLocale())
}

func probeEventDetailViewForLocale(event ProbeEvent, loc *pageLocale) ProbeEventView {
	return ProbeEventView{
		ChangedAt:   event.ChangedAt.Local().Format("2006-01-02 15:04 MST"),
		Entity:      probeEventEntityLabel(event),
		Method:      event.Method,
		Status:      statusTitleForLocale(event.Status, loc),
		StatusClass: probeStatusClass(event.Status),
		Message:     event.Message,
	}
}

func probeEventResponsibleReport(event ProbeLogEvent, reports []*OutageReport, mapping *historyEntityMapping, now time.Time) *OutageReport {
	if isOperationalProbeState(event.Status) || event.ChangedAt.IsZero() || len(reports) == 0 || mapping == nil {
		return nil
	}

	key := historyKey(event.EntityKind, event.EntityID)
	probeStart := event.ChangedAt
	probeEnd := event.EndsAt
	if probeEnd.IsZero() {
		probeEnd = now
	}
	if probeEnd.Before(probeStart) {
		probeEnd = probeStart
	}

	var best *OutageReport
	var bestOverlap time.Duration
	for _, report := range reports {
		if report == nil || report.BeginsAt.IsZero() {
			continue
		}
		if _, ok := mapping.outageHistoryKeys(report)[key]; !ok {
			continue
		}

		reportStart, reportEnd := outageReportInterval(report)
		reportStart = reportStart.Add(-historyReportedIncidentGrace)
		reportEnd = reportEnd.Add(historyReportedIncidentGrace)
		if !historyIntervalsOverlap(probeStart, probeEnd, reportStart, reportEnd) {
			continue
		}

		overlap := minTime(probeEnd, reportEnd).Sub(maxTime(probeStart, reportStart))
		if best == nil ||
			overlap > bestOverlap ||
			(overlap == bestOverlap && report.BeginsAt.After(best.BeginsAt)) ||
			(overlap == bestOverlap && report.BeginsAt.Equal(best.BeginsAt) && report.Id < best.Id) {
			best = report
			bestOverlap = overlap
		}
	}

	return best
}

func probeEventCoverageView(st *Status, report *OutageReport, loc *pageLocale) ProbeEventCoverageView {
	if report == nil {
		return ProbeEventCoverageView{}
	}

	class := "danger"
	if report.IsPlannedOutage() {
		class = "warning"
	}

	return ProbeEventCoverageView{
		ID:    report.Id,
		Label: outageSummaryForLocale(report, loc),
		URL:   outageHistoryIncidentForLocale(st, report, loc).URL,
		Class: class,
	}
}

func setProbeEventCoverageGroups(events []ProbeEventView) {
	for i := range events {
		if !events[i].HasCoverage() {
			continue
		}
		events[i].GroupStart = i == 0 || !sameProbeEventCoverage(events[i-1], events[i])
		events[i].GroupEnd = i == len(events)-1 || !sameProbeEventCoverage(events[i], events[i+1])
	}
}

func sameProbeEventCoverage(a ProbeEventView, b ProbeEventView) bool {
	return a.CoveredBy.ID != 0 && a.CoveredBy.ID == b.CoveredBy.ID
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

func nodeStatusText(node *Node, loc *pageLocale) (string, string) {
	if node.IsOperational() {
		return loc.T("status.operational"), "success"
	}
	if node.IsDegraded() {
		return loc.T("status.degraded"), "warning"
	}
	return loc.T("status.down"), "danger"
}

func dnsResolverStatusText(resolver *DnsResolver, loc *pageLocale) (string, string) {
	if resolver.IsOperational() {
		return loc.T("status.operational"), "success"
	}
	if resolver.IsDegraded() {
		return loc.T("status.degraded"), "warning"
	}
	return loc.T("status.down"), "danger"
}

func webServiceStatusText(ws *WebService, loc *pageLocale) (string, string) {
	switch ws.StatusString() {
	case "operational":
		return loc.T("status.operational"), "success"
	case "maintenance":
		return loc.T("status.under_maintenance"), "warning"
	default:
		if ws.StatusCode != 0 {
			return http.StatusText(ws.StatusCode), "danger"
		}
		return loc.T("status.down"), "danger"
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
	return statusTitleForLocale(status, defaultPageLocale())
}

func statusTitleForLocale(status string, loc *pageLocale) string {
	if status == "" {
		return loc.T("status.unknown")
	}

	switch status {
	case historyProbeStateOperational:
		return loc.T("status.probe.operational")
	case historyProbeStateMaintenance:
		return loc.T("status.probe.maintenance")
	case historyProbeStateDegraded:
		return loc.T("status.probe.degraded")
	case historyProbeStateDown:
		return loc.T("status.probe.down")
	case historyProbeStateError:
		return loc.T("status.probe.error")
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
