package main

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"
)

type HistoryBarView struct {
	Label string
	Days  []HistoryDayView
	Lanes []HistoryLaneView
}

type HistoryDayView struct {
	Date      string
	Label     string
	State     string
	StartsAt  time.Time
	Incidents []HistoryIncidentView
	Lanes     []HistoryLaneView
}

type HistoryIncidentView struct {
	Text          string
	URL           string
	StartLabel    string
	DurationLabel string
}

type HistoryLaneView struct {
	Key   string
	Label string
	State string
}

type historyEntityInfo struct {
	Kind  string
	ID    string
	Label string
}

type historyGroupViews struct {
	VpsAdmin  HistoryBarView
	Locations map[int]HistoryBarView
	Services  HistoryBarView
}

type historyGroupTarget struct {
	Kind       string
	LocationID int
}

type historyData struct {
	days           int
	reports        []*OutageReport
	probeIncidents []ProbeIncident
	snapshots      []HistoryEntitySnapshot
	mapping        *historyEntityMapping
}

type historyArchivedNode struct {
	Key                string
	ID                 string
	Label              string
	GroupID            int
	GroupLabel         string
	NodeID             int64
	VpsAdminLocationID int64
}

type historyEntityMapping struct {
	status                  *Status
	currentNodeKeys         map[string]struct{}
	nodeIDKeys              map[int64]string
	nodeTextKeys            map[string]string
	locationIDNodeKeys      map[int64][]string
	locationTextNodeKeys    map[string][]string
	environmentIDNodeKeys   map[int64][]string
	environmentTextNodeKeys map[string][]string
	dnsResolverTextKeys     map[string]string
	nameServerTextKeys      map[string]string
	webServiceTextKeys      map[string]string
	allNodeKeys             []string
	archivedNodes           map[string]historyArchivedNode
}

const (
	historyGroupVpsAdmin = "vpsadmin"
	historyGroupLocation = "location"
	historyGroupServices = "services"

	historyIncidentTimeFormat    = "2006-01-02 15:04 MST"
	historyReportedIncidentGrace = 30 * time.Minute
)

func (sv *StatusView) HistoryFor(kind string, id string) HistoryBarView {
	if sv == nil || sv.History == nil {
		return HistoryBarView{}
	}

	return sv.History[historyKey(kind, id)]
}

func (sv *StatusView) DetailURL(kind string, id string) string {
	return "/entity?kind=" + url.QueryEscape(kind) + "&id=" + url.QueryEscape(id)
}

func (sv *StatusView) GroupDetailURL(kind string, id any) string {
	ret := "/group?kind=" + url.QueryEscape(kind)
	idString := fmt.Sprint(id)
	if idString != "" {
		ret += "&id=" + url.QueryEscape(idString)
	}
	return ret
}

func (d HistoryDayView) Class() string {
	return historyStateClass(d.State)
}

func (d HistoryDayView) HasLanes() bool {
	return len(d.Lanes) > 0
}

func (l HistoryLaneView) Class() string {
	return historyStateClass(l.State)
}

func (b HistoryBarView) HasLanes() bool {
	return len(b.Lanes) > 0
}

func (b HistoryBarView) DayCount() int {
	return len(b.Days)
}

func (i HistoryIncidentView) HasMetadata() bool {
	return i.StartLabel != "" || i.DurationLabel != ""
}

func (i HistoryIncidentView) Summary() string {
	parts := []string{i.Text}
	if i.StartLabel != "" {
		parts = append(parts, i.StartLabel)
	}
	if i.DurationLabel != "" {
		parts = append(parts, i.DurationLabel)
	}
	return strings.Join(parts, ", ")
}

func historyStateClass(state string) string {
	switch state {
	case historySeverityOutage:
		return "history-day-outage"
	case historySeverityMaintenance:
		return "history-day-maintenance"
	default:
		return "history-day-ok"
	}
}

func (d HistoryDayView) HasIncidents() bool {
	return len(d.Incidents) > 0
}

func (d HistoryDayView) SummaryLabel() string {
	if len(d.Incidents) == 0 {
		return d.Label + ": no incidents"
	}

	summaries := make([]string, len(d.Incidents))
	for i, incident := range d.Incidents {
		summaries[i] = incident.Summary()
	}

	return d.Label + ": " + strings.Join(summaries, "; ")
}

func createHistoryViews(st *Status, now time.Time) (historyGroupViews, map[string]HistoryBarView) {
	return createHistoryViewsWithData(st, now, newHistoryData(st, now))
}

func createHistoryViewsWithData(st *Status, now time.Time, data *historyData) (historyGroupViews, map[string]HistoryBarView) {
	data = ensureHistoryData(st, now, data)
	days := data.days
	entities := configuredHistoryEntities(st)
	bars := make(map[string]HistoryBarView, len(entities))

	for _, entity := range entities {
		bars[historyKey(entity.Kind, entity.ID)] = newHistoryBar(now, entity.Label, days)
	}

	mapping := data.mapping
	reports := data.reports
	probeIncidents := visibleHistoryProbeIncidents(data, now)
	archivedNodes := mapping.archivedNodesForHistoryWindow(reports, probeIncidents, now, days)

	groups := newHistoryGroupViews(st, now, days, archivedNodesByGroup(archivedNodes))
	entityGroups := configuredHistoryEntityGroups(st, archivedNodes)

	for _, report := range reports {
		severity := historySeverityOutage
		if report.IsPlannedOutage() {
			severity = historySeverityMaintenance
		}

		incident := outageHistoryIncident(st, report)

		for key := range mapping.outageHistoryKeys(report) {
			bar, ok := bars[key]
			if ok {
				applyHistoryIncident(&bar, report.BeginsAt, outageEndsAt(report), severity, incident)
				bars[key] = bar
			}

			applyHistoryIncidentToGroups(&groups, entityGroups[key], key, report.BeginsAt, outageEndsAt(report), severity, incident)
		}
	}

	if st != nil && st.History != nil {
		for _, incident := range probeIncidents {
			endsAt := now
			if incident.EndsAt != nil {
				endsAt = *incident.EndsAt
			}

			viewIncident := probeHistoryIncident(incident, now)

			key := historyKey(incident.EntityKind, incident.EntityID)
			bar, ok := bars[key]
			if ok {
				applyHistoryIncident(&bar, incident.StartsAt, endsAt, historySeverityMaintenance, viewIncident)
				bars[key] = bar
			}

			applyHistoryIncidentToGroups(&groups, entityGroups[key], key, incident.StartsAt, endsAt, historySeverityMaintenance, viewIncident)
		}
	}

	return groups, bars
}

func createEntityHistoryView(st *Status, now time.Time, kind string, id string, label string, data *historyData) HistoryBarView {
	data = ensureHistoryData(st, now, data)
	bar := newHistoryBar(now, label, data.days)
	key := historyKey(kind, id)
	probeIncidents := visibleHistoryProbeIncidents(data, now)

	for _, report := range data.reports {
		if _, ok := data.mapping.outageHistoryKeys(report)[key]; !ok {
			continue
		}

		severity := historySeverityOutage
		if report.IsPlannedOutage() {
			severity = historySeverityMaintenance
		}
		applyHistoryIncident(&bar, report.BeginsAt, outageEndsAt(report), severity, outageHistoryIncident(st, report))
	}

	for _, incident := range probeIncidents {
		if historyKey(incident.EntityKind, incident.EntityID) != key {
			continue
		}

		endsAt := now
		if incident.EndsAt != nil {
			endsAt = *incident.EndsAt
		}
		applyHistoryIncident(&bar, incident.StartsAt, endsAt, historySeverityMaintenance, probeHistoryIncident(incident, now))
	}

	return bar
}

func createGroupHistoryView(st *Status, now time.Time, target historyGroupTarget, label string, data *historyData) HistoryBarView {
	data = ensureHistoryData(st, now, data)

	probeIncidents := visibleHistoryProbeIncidents(data, now)
	archivedNodes := data.mapping.archivedNodesForHistoryWindow(data.reports, probeIncidents, now, data.days)
	entityGroups := configuredHistoryEntityGroups(st, archivedNodes)

	var lanes []HistoryLaneView
	switch target.Kind {
	case historyGroupVpsAdmin:
		lanes = vpsAdminHistoryLanes()
	case historyGroupLocation:
		if loc := findLocationByID(st, target.LocationID); loc != nil {
			lanes = locationHistoryLanes(loc, archivedNodesByGroup(archivedNodes)[target.LocationID])
		}
	case historyGroupServices:
		if st != nil {
			lanes = servicesHistoryLanes(st.Services)
		}
	}

	bar := newHistoryBarWithLanes(now, label, lanes, data.days)

	for _, report := range data.reports {
		severity := historySeverityOutage
		if report.IsPlannedOutage() {
			severity = historySeverityMaintenance
		}
		incident := outageHistoryIncident(st, report)

		for key := range data.mapping.outageHistoryKeys(report) {
			if !historyTargetsInclude(entityGroups[key], target) {
				continue
			}
			applyHistoryIncidentToLane(&bar, key, report.BeginsAt, outageEndsAt(report), severity, incident)
		}
	}

	for _, incident := range probeIncidents {
		key := historyKey(incident.EntityKind, incident.EntityID)
		if !historyTargetsInclude(entityGroups[key], target) {
			continue
		}

		endsAt := now
		if incident.EndsAt != nil {
			endsAt = *incident.EndsAt
		}
		applyHistoryIncidentToLane(&bar, key, incident.StartsAt, endsAt, historySeverityMaintenance, probeHistoryIncident(incident, now))
	}

	return bar
}

func historyTargetsInclude(targets []historyGroupTarget, want historyGroupTarget) bool {
	for _, target := range targets {
		if target.Kind != want.Kind {
			continue
		}
		if target.Kind == historyGroupLocation && target.LocationID != want.LocationID {
			continue
		}
		return true
	}
	return false
}

func configuredHistoryEntities(st *Status) []historyEntityInfo {
	if st == nil {
		return nil
	}

	ret := []historyEntityInfo{
		{Kind: historyEntityVpsAdmin, ID: "webui", Label: "vpsAdmin web UI"},
		{Kind: historyEntityVpsAdmin, ID: "api", Label: "vpsAdmin API"},
		{Kind: historyEntityVpsAdmin, ID: "console", Label: "Remote Console"},
	}

	for _, loc := range st.LocationList {
		for _, node := range loc.NodeList {
			ret = append(ret, historyEntityInfo{
				Kind:  historyEntityNode,
				ID:    node.Name,
				Label: node.Name,
			})
		}

		for _, resolver := range loc.DnsResolverList {
			ret = append(ret, historyEntityInfo{
				Kind:  historyEntityDnsResolver,
				ID:    resolver.Name,
				Label: resolver.Name,
			})
		}
	}

	for _, ws := range st.Services.Web {
		ret = append(ret, historyEntityInfo{
			Kind:  historyEntityWebService,
			ID:    ws.Label,
			Label: ws.Label,
		})
	}

	for _, ns := range st.Services.NameServer {
		ret = append(ret, historyEntityInfo{
			Kind:  historyEntityNameServer,
			ID:    ns.Name,
			Label: ns.Name,
		})
	}

	return ret
}

func newHistoryGroupViews(st *Status, now time.Time, days int, archivedByGroup map[int][]historyArchivedNode) historyGroupViews {
	ret := historyGroupViews{
		VpsAdmin:  newHistoryBarWithLanes(now, "vpsAdmin", vpsAdminHistoryLanes(), days),
		Locations: make(map[int]HistoryBarView),
	}
	if st == nil {
		ret.Services = newHistoryBarWithLanes(now, "Services", nil, days)
		return ret
	}

	for _, loc := range st.LocationList {
		ret.Locations[loc.Id] = newHistoryBarWithLanes(now, loc.Label, locationHistoryLanes(loc, archivedByGroup[loc.Id]), days)
	}
	ret.Services = newHistoryBarWithLanes(now, "Services", servicesHistoryLanes(st.Services), days)

	return ret
}

func vpsAdminHistoryLanes() []HistoryLaneView {
	return []HistoryLaneView{
		newHistoryLane(historyEntityVpsAdmin, "webui", "vpsAdmin web UI"),
		newHistoryLane(historyEntityVpsAdmin, "api", "vpsAdmin API"),
		newHistoryLane(historyEntityVpsAdmin, "console", "Remote Console"),
	}
}

func locationHistoryLanes(loc *Location, archivedNodes []historyArchivedNode) []HistoryLaneView {
	if loc == nil {
		return nil
	}

	ret := make([]HistoryLaneView, 0, len(loc.NodeList)+len(archivedNodes)+len(loc.DnsResolverList))
	for _, node := range loc.NodeList {
		ret = append(ret, newHistoryLane(historyEntityNode, node.Name, node.Name))
	}
	for _, node := range archivedNodes {
		ret = append(ret, HistoryLaneView{
			Key:   node.Key,
			Label: node.Label,
			State: historySeverityOperational,
		})
	}
	for _, resolver := range loc.DnsResolverList {
		ret = append(ret, newHistoryLane(historyEntityDnsResolver, resolver.Name, resolver.Name))
	}
	return ret
}

func servicesHistoryLanes(services *Services) []HistoryLaneView {
	if services == nil {
		return nil
	}

	ret := make([]HistoryLaneView, 0, len(services.Web)+len(services.NameServer))
	for _, ws := range services.Web {
		ret = append(ret, newHistoryLane(historyEntityWebService, ws.Label, ws.Label))
	}
	for _, ns := range services.NameServer {
		ret = append(ret, newHistoryLane(historyEntityNameServer, ns.Name, ns.Name))
	}
	return ret
}

func newHistoryLane(kind string, id string, label string) HistoryLaneView {
	return HistoryLaneView{
		Key:   historyKey(kind, id),
		Label: label,
		State: historySeverityOperational,
	}
}

func configuredHistoryEntityGroups(st *Status, archivedNodes []historyArchivedNode) map[string][]historyGroupTarget {
	ret := make(map[string][]historyGroupTarget)
	if st == nil {
		return ret
	}

	vpsAdminTarget := historyGroupTarget{Kind: historyGroupVpsAdmin}
	for _, id := range []string{"webui", "api", "console"} {
		addHistoryGroupTarget(ret, historyKey(historyEntityVpsAdmin, id), vpsAdminTarget)
	}

	for _, loc := range st.LocationList {
		target := historyGroupTarget{Kind: historyGroupLocation, LocationID: loc.Id}
		for _, node := range loc.NodeList {
			addHistoryGroupTarget(ret, historyKey(historyEntityNode, node.Name), target)
		}
		for _, resolver := range loc.DnsResolverList {
			addHistoryGroupTarget(ret, historyKey(historyEntityDnsResolver, resolver.Name), target)
		}
	}
	for _, node := range archivedNodes {
		addHistoryGroupTarget(ret, node.Key, historyGroupTarget{Kind: historyGroupLocation, LocationID: node.GroupID})
	}

	servicesTarget := historyGroupTarget{Kind: historyGroupServices}
	for _, ws := range st.Services.Web {
		addHistoryGroupTarget(ret, historyKey(historyEntityWebService, ws.Label), servicesTarget)
	}
	for _, ns := range st.Services.NameServer {
		addHistoryGroupTarget(ret, historyKey(historyEntityNameServer, ns.Name), servicesTarget)
	}

	return ret
}

func addHistoryGroupTarget(groups map[string][]historyGroupTarget, key string, target historyGroupTarget) {
	groups[key] = append(groups[key], target)
}

func applyHistoryIncidentToGroups(groups *historyGroupViews, targets []historyGroupTarget, laneKey string, startsAt time.Time, endsAt time.Time, severity string, incident HistoryIncidentView) {
	if groups == nil {
		return
	}

	for _, target := range targets {
		switch target.Kind {
		case historyGroupVpsAdmin:
			bar := groups.VpsAdmin
			applyHistoryIncidentToLane(&bar, laneKey, startsAt, endsAt, severity, incident)
			groups.VpsAdmin = bar
		case historyGroupLocation:
			bar, ok := groups.Locations[target.LocationID]
			if !ok {
				continue
			}
			applyHistoryIncidentToLane(&bar, laneKey, startsAt, endsAt, severity, incident)
			groups.Locations[target.LocationID] = bar
		case historyGroupServices:
			bar := groups.Services
			applyHistoryIncidentToLane(&bar, laneKey, startsAt, endsAt, severity, incident)
			groups.Services = bar
		}
	}
}

func newHistoryBar(now time.Time, label string, days int) HistoryBarView {
	return newHistoryBarWithLanes(now, label, nil, days)
}

func newHistoryBarWithLanes(now time.Time, label string, lanes []HistoryLaneView, dayCount int) HistoryBarView {
	dayCount = historyWindowDays(dayCount)
	start := historyStartDay(now, dayCount)
	days := make([]HistoryDayView, dayCount)
	cleanLanes := cloneHistoryLanes(lanes)

	for i := range days {
		day := start.AddDate(0, 0, i)
		days[i] = HistoryDayView{
			Date:     day.Format("2006-01-02"),
			Label:    day.Format("Jan _2, 2006"),
			State:    historySeverityOperational,
			StartsAt: day,
			Lanes:    cloneHistoryLanes(cleanLanes),
		}
	}

	return HistoryBarView{
		Label: label,
		Days:  days,
		Lanes: cleanLanes,
	}
}

func applyHistoryIncident(bar *HistoryBarView, startsAt time.Time, endsAt time.Time, severity string, incident HistoryIncidentView) {
	if bar == nil || startsAt.IsZero() || incident.Text == "" {
		return
	}

	if endsAt.IsZero() || endsAt.Before(startsAt) {
		endsAt = startsAt
	}

	startDay := localDay(startsAt)
	endDay := localDay(endsAt)

	for i := range bar.Days {
		day := bar.Days[i].StartsAt
		if day.Before(startDay) || day.After(endDay) {
			continue
		}

		if severity == historySeverityOutage {
			bar.Days[i].State = historySeverityOutage
		} else if bar.Days[i].State != historySeverityOutage {
			bar.Days[i].State = historySeverityMaintenance
		}

		if !containsHistoryIncident(bar.Days[i].Incidents, incident) {
			bar.Days[i].Incidents = append(bar.Days[i].Incidents, incident)
		}
	}
}

func applyHistoryIncidentToLane(bar *HistoryBarView, laneKey string, startsAt time.Time, endsAt time.Time, severity string, incident HistoryIncidentView) {
	if bar == nil || laneKey == "" || startsAt.IsZero() || incident.Text == "" {
		return
	}
	if len(bar.Lanes) == 0 {
		applyHistoryIncident(bar, startsAt, endsAt, severity, incident)
		return
	}

	if endsAt.IsZero() || endsAt.Before(startsAt) {
		endsAt = startsAt
	}

	startDay := localDay(startsAt)
	endDay := localDay(endsAt)

	for i := range bar.Days {
		day := bar.Days[i].StartsAt
		if day.Before(startDay) || day.After(endDay) {
			continue
		}

		laneLabel, ok := setHistoryLaneSeverity(bar.Days[i].Lanes, laneKey, severity)
		if !ok {
			continue
		}

		if severity == historySeverityOutage {
			bar.Days[i].State = historySeverityOutage
		} else if bar.Days[i].State != historySeverityOutage {
			bar.Days[i].State = historySeverityMaintenance
		}

		laneIncident := historyIncidentWithLaneLabel(incident, laneLabel)
		if !containsHistoryIncident(bar.Days[i].Incidents, laneIncident) {
			bar.Days[i].Incidents = append(bar.Days[i].Incidents, laneIncident)
		}
	}
}

func setHistoryLaneSeverity(lanes []HistoryLaneView, laneKey string, severity string) (string, bool) {
	for i := range lanes {
		if lanes[i].Key != laneKey {
			continue
		}

		if severity == historySeverityOutage {
			lanes[i].State = historySeverityOutage
		} else if lanes[i].State != historySeverityOutage {
			lanes[i].State = historySeverityMaintenance
		}
		return lanes[i].Label, true
	}

	return "", false
}

func historyIncidentWithLaneLabel(incident HistoryIncidentView, label string) HistoryIncidentView {
	if label == "" || strings.Contains(normalizeEntityText(incident.Text), normalizeEntityText(label)) {
		return incident
	}

	ret := incident
	ret.Text = label + ": " + ret.Text
	return ret
}

func cloneHistoryLanes(lanes []HistoryLaneView) []HistoryLaneView {
	if len(lanes) == 0 {
		return nil
	}

	ret := make([]HistoryLaneView, len(lanes))
	copy(ret, lanes)
	for i := range ret {
		if ret[i].State == "" {
			ret[i].State = historySeverityOperational
		}
	}
	return ret
}

func historyOutageReports(st *Status) []*OutageReport {
	if st == nil {
		return nil
	}

	if st.History != nil {
		reports := st.History.OutageReports()
		if len(reports) > 0 {
			return reports
		}
	}

	ret := make([]*OutageReport, 0)
	if st.OutageReports != nil {
		ret = append(ret, st.OutageReports.ActiveList...)
		ret = append(ret, st.OutageReports.RecentList...)
	}
	return ret
}

func newHistoryData(st *Status, now time.Time) *historyData {
	data := &historyData{
		days: historyDaysForStatus(st),
	}

	if st != nil && st.History != nil {
		data.reports = st.History.OutageReports()
		data.probeIncidents = st.History.ProbeIncidents(now, data.days)
		data.snapshots = st.History.EntitySnapshots()
	}
	if len(data.reports) == 0 && st != nil && st.OutageReports != nil {
		data.reports = append(data.reports, st.OutageReports.ActiveList...)
		data.reports = append(data.reports, st.OutageReports.RecentList...)
	}

	data.mapping = newHistoryEntityMappingWithSnapshots(st, data.snapshots)
	return data
}

func ensureHistoryData(st *Status, now time.Time, data *historyData) *historyData {
	if data != nil {
		return data
	}
	return newHistoryData(st, now)
}

func visibleHistoryProbeIncidents(data *historyData, now time.Time) []ProbeIncident {
	if data == nil {
		return nil
	}
	return filterCoveredHistoryProbeIncidents(data.probeIncidents, data.reports, data.mapping, now)
}

func filterCoveredHistoryProbeIncidents(incidents []ProbeIncident, reports []*OutageReport, mapping *historyEntityMapping, now time.Time) []ProbeIncident {
	if len(incidents) == 0 || len(reports) == 0 || mapping == nil {
		return incidents
	}

	ret := make([]ProbeIncident, 0, len(incidents))
	for _, incident := range incidents {
		if historyProbeIncidentCoveredByReport(incident, reports, mapping, now) {
			continue
		}
		ret = append(ret, incident)
	}
	return ret
}

func historyProbeIncidentCoveredByReport(incident ProbeIncident, reports []*OutageReport, mapping *historyEntityMapping, now time.Time) bool {
	if incident.EntityKind == "" || incident.EntityID == "" || incident.StartsAt.IsZero() {
		return false
	}

	key := historyKey(incident.EntityKind, incident.EntityID)
	probeStart, probeEnd := probeIncidentInterval(incident, now)
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
		if historyIntervalsOverlap(probeStart, probeEnd, reportStart, reportEnd) {
			return true
		}
	}
	return false
}

func probeIncidentInterval(incident ProbeIncident, now time.Time) (time.Time, time.Time) {
	start := incident.StartsAt
	end := now
	if incident.EndsAt != nil {
		end = *incident.EndsAt
	}
	if end.Before(start) {
		end = start
	}
	return start, end
}

func outageReportInterval(report *OutageReport) (time.Time, time.Time) {
	start := report.BeginsAt
	end := outageEndsAt(report)
	if end.IsZero() || end.Before(start) {
		end = start
	}
	return start, end
}

func historyIntervalsOverlap(aStart time.Time, aEnd time.Time, bStart time.Time, bEnd time.Time) bool {
	return !aEnd.Before(bStart) && !bEnd.Before(aStart)
}

func outageHistoryKeys(st *Status, report *OutageReport) map[string]struct{} {
	return newHistoryEntityMapping(st).outageHistoryKeys(report)
}

func newHistoryEntityMapping(st *Status) *historyEntityMapping {
	var snapshots []HistoryEntitySnapshot
	if st != nil && st.History != nil {
		snapshots = st.History.EntitySnapshots()
	}
	return newHistoryEntityMappingWithSnapshots(st, snapshots)
}

func newHistoryEntityMappingWithSnapshots(st *Status, snapshots []HistoryEntitySnapshot) *historyEntityMapping {
	ret := &historyEntityMapping{
		status:                  st,
		currentNodeKeys:         make(map[string]struct{}),
		nodeIDKeys:              make(map[int64]string),
		nodeTextKeys:            make(map[string]string),
		locationIDNodeKeys:      make(map[int64][]string),
		locationTextNodeKeys:    make(map[string][]string),
		environmentIDNodeKeys:   make(map[int64][]string),
		environmentTextNodeKeys: make(map[string][]string),
		dnsResolverTextKeys:     make(map[string]string),
		nameServerTextKeys:      make(map[string]string),
		webServiceTextKeys:      make(map[string]string),
		allNodeKeys:             make([]string, 0),
		archivedNodes:           make(map[string]historyArchivedNode),
	}
	if st == nil {
		return ret
	}

	for _, loc := range st.LocationList {
		for _, node := range loc.NodeList {
			key := historyKey(historyEntityNode, node.Name)
			ret.currentNodeKeys[key] = struct{}{}

			locationID := int64(node.LocationId)
			if locationID == 0 {
				locationID = int64(loc.Id)
			}
			ret.addNodeMapping(key, int64(node.Id), node.Name, locationID)
		}

		for _, resolver := range loc.DnsResolverList {
			key := historyKey(historyEntityDnsResolver, resolver.Name)
			ret.addTextHistoryMapping(ret.dnsResolverTextKeys, key, resolver.Name, resolver.IpAddress)
		}
	}

	for _, ws := range st.Services.Web {
		key := historyKey(historyEntityWebService, ws.Label)
		ret.addTextHistoryMapping(ret.webServiceTextKeys, key, ws.Label, ws.Url, ws.CheckUrl, hostText(ws.Url), hostText(ws.CheckUrl))
	}

	for _, ns := range st.Services.NameServer {
		key := historyKey(historyEntityNameServer, ns.Name)
		ret.addTextHistoryMapping(ret.nameServerTextKeys, key, ns.Name, ns.IpAddress)
	}

	for _, snapshot := range snapshots {
		if snapshot.EntityKind != historyEntityNode || snapshot.EntityID == "" {
			continue
		}

		key := historyKey(historyEntityNode, snapshot.EntityID)
		if _, ok := ret.currentNodeKeys[key]; ok {
			continue
		}

		node, ok := ret.archivedNodeFromSnapshot(snapshot)
		if !ok {
			continue
		}

		ret.archivedNodes[key] = node
		ret.addNodeMapping(key, snapshot.NodeID, snapshot.EntityID, snapshot.VpsAdminLocationID)
		ret.addNodeMapping(key, 0, snapshot.EntityLabel, snapshot.VpsAdminLocationID)
		if snapshot.GroupLabel != "" {
			ret.locationTextNodeKeys[normalizeEntityText(snapshot.GroupLabel)] = addUniqueHistoryKey(ret.locationTextNodeKeys[normalizeEntityText(snapshot.GroupLabel)], key)
		}
	}

	ret.addLocationAndEnvironmentMappings()
	return ret
}

func (m *historyEntityMapping) addNodeMapping(key string, nodeID int64, text string, locationID int64) {
	if key == "" {
		return
	}

	m.allNodeKeys = addUniqueHistoryKey(m.allNodeKeys, key)
	if nodeID != 0 {
		m.nodeIDKeys[nodeID] = key
	}
	if text != "" {
		m.nodeTextKeys[normalizeEntityText(text)] = key
	}
	if locationID != 0 {
		m.locationIDNodeKeys[locationID] = addUniqueHistoryKey(m.locationIDNodeKeys[locationID], key)
	}
}

func (m *historyEntityMapping) addTextHistoryMapping(dst map[string]string, key string, labels ...string) {
	for _, label := range labels {
		normalized := normalizeEntityText(label)
		if normalized == "" {
			continue
		}
		if _, exists := dst[normalized]; !exists {
			dst[normalized] = key
		}
	}
}

func (m *historyEntityMapping) addLocationAndEnvironmentMappings() {
	if m == nil || m.status == nil {
		return
	}

	if len(m.status.VpsAdminLocations) > 0 {
		for _, loc := range m.status.VpsAdminLocations {
			keys := m.locationIDNodeKeys[loc.Id]
			if len(keys) == 0 {
				continue
			}

			m.locationTextNodeKeys[normalizeEntityText(loc.Label)] = addUniqueHistoryKeys(m.locationTextNodeKeys[normalizeEntityText(loc.Label)], keys)

			if loc.EnvironmentId != 0 {
				m.environmentIDNodeKeys[loc.EnvironmentId] = addUniqueHistoryKeys(m.environmentIDNodeKeys[loc.EnvironmentId], keys)
			}
			if loc.EnvironmentLabel != "" {
				m.environmentTextNodeKeys[normalizeEntityText(loc.EnvironmentLabel)] = addUniqueHistoryKeys(m.environmentTextNodeKeys[normalizeEntityText(loc.EnvironmentLabel)], keys)
			}
		}
		return
	}

	for _, loc := range m.status.LocationList {
		keys := make([]string, 0, len(loc.NodeList))
		for _, node := range loc.NodeList {
			keys = append(keys, historyKey(historyEntityNode, node.Name))
		}
		m.locationTextNodeKeys[normalizeEntityText(loc.Label)] = addUniqueHistoryKeys(m.locationTextNodeKeys[normalizeEntityText(loc.Label)], keys)
	}
}

func (m *historyEntityMapping) archivedNodeFromSnapshot(snapshot HistoryEntitySnapshot) (historyArchivedNode, bool) {
	groupID, groupLabel, ok := m.resolveExistingGroup(snapshot.GroupID, snapshot.GroupLabel)
	if !ok {
		groupID, groupLabel, ok = m.inferGroupForNode(snapshot.EntityID)
	}
	if !ok {
		return historyArchivedNode{}, false
	}

	label := snapshot.EntityLabel
	if label == "" {
		label = snapshot.EntityID
	}

	return historyArchivedNode{
		Key:                historyKey(historyEntityNode, snapshot.EntityID),
		ID:                 snapshot.EntityID,
		Label:              label + " (removed)",
		GroupID:            groupID,
		GroupLabel:         groupLabel,
		NodeID:             snapshot.NodeID,
		VpsAdminLocationID: snapshot.VpsAdminLocationID,
	}, true
}

func (m *historyEntityMapping) archivedNodeForKey(key string) (historyArchivedNode, bool) {
	if node, ok := m.archivedNodes[key]; ok {
		return node, true
	}
	if _, ok := m.currentNodeKeys[key]; ok {
		return historyArchivedNode{}, false
	}

	kind, id, ok := splitHistoryKey(key)
	if !ok || kind != historyEntityNode || id == "" {
		return historyArchivedNode{}, false
	}

	groupID, groupLabel, ok := m.inferGroupForNode(id)
	if !ok {
		return historyArchivedNode{}, false
	}

	return historyArchivedNode{
		Key:        key,
		ID:         id,
		Label:      id + " (removed)",
		GroupID:    groupID,
		GroupLabel: groupLabel,
	}, true
}

func (m *historyEntityMapping) resolveExistingGroup(groupID int, groupLabel string) (int, string, bool) {
	if m == nil || m.status == nil {
		return 0, "", false
	}

	for _, loc := range m.status.LocationList {
		if groupID != 0 && loc.Id == groupID {
			return loc.Id, loc.Label, true
		}
	}
	for _, loc := range m.status.LocationList {
		if groupLabel != "" && normalizeEntityText(loc.Label) == normalizeEntityText(groupLabel) {
			return loc.Id, loc.Label, true
		}
	}
	return 0, "", false
}

func (m *historyEntityMapping) inferGroupForNode(id string) (int, string, bool) {
	groupLabel := ""
	switch {
	case strings.HasSuffix(id, ".brq"):
		groupLabel = "Brno"
	case strings.HasSuffix(id, ".prg"), strings.HasSuffix(id, ".stg"), strings.HasSuffix(id, ".pgnd"):
		groupLabel = "Praha"
	default:
		return 0, "", false
	}

	return m.resolveExistingGroup(0, groupLabel)
}

func (m *historyEntityMapping) archivedNodesForHistoryWindow(reports []*OutageReport, probeIncidents []ProbeIncident, now time.Time, days int) []historyArchivedNode {
	if m == nil {
		return nil
	}

	nodes := make(map[string]historyArchivedNode)
	for _, report := range reports {
		if !outageOverlapsHistoryWindow(report, now, days) {
			continue
		}

		for key := range m.outageHistoryKeys(report) {
			if node, ok := m.archivedNodeForKey(key); ok {
				nodes[key] = node
			}
		}
	}

	for _, incident := range probeIncidents {
		if incident.EntityKind != historyEntityNode {
			continue
		}

		key := historyKey(incident.EntityKind, incident.EntityID)
		if node, ok := m.archivedNodeForKey(key); ok {
			nodes[key] = node
		}
	}

	ret := make([]historyArchivedNode, 0, len(nodes))
	for _, node := range nodes {
		ret = append(ret, node)
	}
	sort.Slice(ret, func(i, j int) bool {
		if ret[i].GroupID != ret[j].GroupID {
			return ret[i].GroupID < ret[j].GroupID
		}
		if ret[i].Label != ret[j].Label {
			return ret[i].Label < ret[j].Label
		}
		return ret[i].ID < ret[j].ID
	})
	return ret
}

func (m *historyEntityMapping) outageHistoryKeys(report *OutageReport) map[string]struct{} {
	ret := make(map[string]struct{})
	if m == nil || report == nil {
		return ret
	}

	for _, entity := range report.AffectedEntities {
		name := normalizeEntityText(entity.Name)
		label := normalizeEntityText(entity.Label)

		if name == "cluster" {
			addHistoryKeys(ret, m.allNodeKeys)
			continue
		}

		if name == "node" {
			if key, ok := m.nodeIDKeys[entity.Id]; ok {
				ret[key] = struct{}{}
				continue
			}
		}

		if key, ok := m.nodeTextKeys[label]; ok {
			ret[key] = struct{}{}
			continue
		}
		if name == "node" && label != "" {
			ret[historyKey(historyEntityNode, label)] = struct{}{}
			continue
		}

		if name == "location" {
			if keys, ok := m.locationIDNodeKeys[entity.Id]; ok {
				for _, key := range keys {
					ret[key] = struct{}{}
				}
				continue
			}
		}

		if keys, ok := m.locationTextNodeKeys[label]; ok {
			addHistoryKeys(ret, keys)
			continue
		}

		if name == "environment" {
			if keys, ok := m.environmentIDNodeKeys[entity.Id]; ok {
				addHistoryKeys(ret, keys)
				continue
			}
		}

		if keys, ok := m.environmentTextNodeKeys[label]; ok {
			addHistoryKeys(ret, keys)
			continue
		}

		if key, ok := m.serviceHistoryKey(entity, m.dnsResolverTextKeys); ok {
			ret[key] = struct{}{}
			continue
		}
		if key, ok := m.serviceHistoryKey(entity, m.nameServerTextKeys); ok {
			ret[key] = struct{}{}
			continue
		}
		if key, ok := m.serviceHistoryKey(entity, m.webServiceTextKeys); ok {
			ret[key] = struct{}{}
			continue
		}

		for _, key := range vpsAdminServiceHistoryKeys(m.status, entity) {
			ret[key] = struct{}{}
		}
	}

	return ret
}

func (m *historyEntityMapping) serviceHistoryKey(entity OutageEntity, keys map[string]string) (string, bool) {
	for _, candidate := range []string{entity.Label, entity.Name} {
		if key, ok := keys[normalizeEntityText(candidate)]; ok {
			return key, true
		}
	}

	text := normalizeEntityText(entity.Name + " " + entity.Label)
	for label, key := range keys {
		if label != "" && strings.Contains(text, label) {
			return key, true
		}
	}

	return "", false
}

func archivedNodesByGroup(nodes []historyArchivedNode) map[int][]historyArchivedNode {
	ret := make(map[int][]historyArchivedNode)
	for _, node := range nodes {
		ret[node.GroupID] = append(ret[node.GroupID], node)
	}
	return ret
}

func outageOverlapsHistoryWindow(report *OutageReport, now time.Time, days int) bool {
	if report == nil || report.BeginsAt.IsZero() {
		return false
	}

	windowStart := historyStartDay(now, days)
	windowEnd := localDay(now)
	return !outageEndsAt(report).Before(windowStart) && !localDay(report.BeginsAt).After(windowEnd)
}

func addHistoryKeys(dst map[string]struct{}, keys []string) {
	for _, key := range keys {
		dst[key] = struct{}{}
	}
}

func addUniqueHistoryKeys(dst []string, keys []string) []string {
	for _, key := range keys {
		dst = addUniqueHistoryKey(dst, key)
	}
	return dst
}

func addUniqueHistoryKey(dst []string, key string) []string {
	if key == "" {
		return dst
	}
	for _, existing := range dst {
		if existing == key {
			return dst
		}
	}
	return append(dst, key)
}

func splitHistoryKey(key string) (string, string, bool) {
	parts := strings.SplitN(key, "\t", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func vpsAdminServiceHistoryKeys(st *Status, entity OutageEntity) []string {
	if st == nil {
		return nil
	}

	text := normalizeEntityText(entity.Name + " " + entity.Label)
	if text == "" {
		return nil
	}

	services := []struct {
		id     string
		labels []string
	}{
		{id: "api", labels: []string{"api", hostText(st.VpsAdmin.Api.Url), normalizeEntityText(st.VpsAdmin.Api.Label)}},
		{id: "webui", labels: []string{"webui", "web ui", "vpsadmin", hostText(st.VpsAdmin.Webui.Url), normalizeEntityText(st.VpsAdmin.Webui.Label)}},
		{id: "console", labels: []string{"console", "remote console", hostText(st.VpsAdmin.Console.Url), normalizeEntityText(st.VpsAdmin.Console.Label)}},
	}

	var ret []string
	for _, service := range services {
		for _, label := range service.labels {
			if label == "" {
				continue
			}
			if strings.Contains(text, label) {
				ret = append(ret, historyKey(historyEntityVpsAdmin, service.id))
				break
			}
		}
	}

	return ret
}

func outageSummary(report *OutageReport) string {
	if report == nil {
		return ""
	}

	summary := report.EnSummary
	if summary == "" {
		summary = report.CsSummary
	}
	if summary == "" {
		summary = fmt.Sprintf("Outage #%d", report.Id)
	}

	if report.IsPlannedOutage() {
		return "Planned outage: " + summary
	}

	return "Unplanned outage: " + summary
}

func outageHistoryIncident(st *Status, report *OutageReport) HistoryIncidentView {
	ret := HistoryIncidentView{Text: outageSummary(report)}
	if report != nil {
		ret.StartLabel = historyIncidentStartLabel(report.BeginsAt)
		ret.DurationLabel = historyIncidentDurationLabel("Expected duration", report.Duration, false)
	}
	if st == nil || report == nil || report.Id == 0 || st.VpsAdmin.Webui == nil || st.VpsAdmin.Webui.Url == "" {
		return ret
	}

	ret.URL = fmt.Sprintf(
		"%s/?page=outage&action=show&id=%d",
		strings.TrimRight(st.VpsAdmin.Webui.Url, "/"),
		report.Id,
	)
	return ret
}

func probeHistoryIncident(incident ProbeIncident, now time.Time) HistoryIncidentView {
	label := incident.EntityLabel
	if label == "" {
		label = incident.EntityID
	}

	status := incident.Status
	if status == "" {
		status = historyProbeStateDown
	}

	message := incident.Message
	if message == "" {
		message = status
	}

	endsAt := now
	open := incident.EndsAt == nil
	if incident.EndsAt != nil {
		endsAt = *incident.EndsAt
	}
	if endsAt.Before(incident.StartsAt) {
		endsAt = incident.StartsAt
	}

	return HistoryIncidentView{
		Text:          fmt.Sprintf("Probe: %s %s %s", label, incident.Method, message),
		StartLabel:    historyIncidentStartLabel(incident.StartsAt),
		DurationLabel: historyIncidentDurationLabel("Observed duration", endsAt.Sub(incident.StartsAt), open),
	}
}

func historyIncidentStartLabel(t time.Time) string {
	if t.IsZero() {
		return ""
	}

	return "Started: " + t.Local().Format(historyIncidentTimeFormat)
}

func historyIncidentDurationLabel(label string, duration time.Duration, soFar bool) string {
	if duration < 0 {
		duration = 0
	}

	ret := fmt.Sprintf("%s: %d min", label, int(duration/time.Minute))
	if soFar {
		ret += " so far"
	}
	return ret
}

func localDay(t time.Time) time.Time {
	local := t.Local()
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, local.Location())
}

func normalizeEntityText(s string) string {
	ret := strings.ToLower(strings.TrimSpace(s))
	ret = strings.TrimPrefix(ret, "node ")
	ret = strings.TrimPrefix(ret, "location ")
	ret = strings.TrimPrefix(ret, "environment ")
	ret = strings.TrimPrefix(ret, "cluster ")
	return strings.TrimSpace(ret)
}

func hostText(rawurl string) string {
	if rawurl == "" {
		return ""
	}

	u, err := url.Parse(rawurl)
	if err != nil || u.Host == "" {
		return normalizeEntityText(rawurl)
	}

	return normalizeEntityText(u.Hostname())
}

func containsHistoryIncident(values []HistoryIncidentView, needle HistoryIncidentView) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
