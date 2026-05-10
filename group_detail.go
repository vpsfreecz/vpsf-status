package main

import (
	"sort"
	"strconv"
	"time"
)

func createGroupDetailView(st *Status, kind string, id string, now time.Time, probePage int) (EntityDetailView, bool) {
	if st == nil || kind == "" {
		return EntityDetailView{}, false
	}

	ret := EntityDetailView{
		Kind:            kind,
		ID:              id,
		Group:           "Group",
		ShowEventEntity: true,
	}

	var groupTarget historyGroupTarget
	var probeTargets []historyEntityInfo
	var reportedTargets []historyEntityInfo

	switch kind {
	case historyGroupVpsAdmin:
		view := createVpsAdminView(st.VpsAdmin)
		ret.Label = "vpsAdmin"
		ret.StatusText, ret.StatusClass = groupStatusText(view.IsOperational(), view.IsDegraded())
		ret.ShowReportedAvailability = true
		groupTarget = historyGroupTarget{Kind: historyGroupVpsAdmin}
		probeTargets = vpsAdminGroupTargets()
		reportedTargets = probeTargets
	case historyGroupLocation:
		locID, err := strconv.Atoi(id)
		if err != nil {
			return EntityDetailView{}, false
		}
		loc := findLocationByID(st, locID)
		if loc == nil {
			return EntityDetailView{}, false
		}

		view := createLocationView([]*Location{loc})[0]
		ret.Label = loc.Label
		ret.Group = "Location"
		ret.StatusText, ret.StatusClass = groupStatusText(view.IsOperational(), view.IsDegraded())
		ret.ShowReportedAvailability = true
		groupTarget = historyGroupTarget{Kind: historyGroupLocation, LocationID: loc.Id}
		probeTargets = locationGroupTargets(loc)
		reportedTargets = locationReportedTargets(loc)
	case historyGroupServices:
		view := createServicesView(st.Services)
		ret.Label = "Services"
		ret.StatusText, ret.StatusClass = groupStatusText(view.IsOperational(), view.IsDegraded())
		groupTarget = historyGroupTarget{Kind: historyGroupServices}
		probeTargets = servicesGroupTargets(st.Services)
	default:
		return EntityDetailView{}, false
	}

	data := newHistoryData(st, now)
	ret.History = createGroupHistoryView(st, now, groupTarget, ret.Label, data)
	ret.Availability = availabilityDetailViews(groupAvailabilityWithData(st, probeTargets, reportedTargets, now, data))
	logPage, page := paginatedProbeLog(probePage, func(limit int, offset int) ProbeLogPage {
		return groupProbeLog(st, probeTargets, now, limit, offset)
	})
	ret.Events = probeLogEventDetailViews(st, logPage.Events, now, data)
	ret.EventPagination = newProbeLogPaginationView("/group", kind, id, page, logPage.Total)
	return ret, true
}

func groupStatusText(operational bool, degraded bool) (string, string) {
	if operational {
		return "Operational", "success"
	}
	if degraded {
		return "Degraded", "warning"
	}
	return "Down", "danger"
}

func findLocationByID(st *Status, id int) *Location {
	if st == nil {
		return nil
	}
	for _, loc := range st.LocationList {
		if loc.Id == id {
			return loc
		}
	}
	return nil
}

func vpsAdminGroupTargets() []historyEntityInfo {
	return []historyEntityInfo{
		{Kind: historyEntityVpsAdmin, ID: "webui", Label: vpsAdminServiceLabel("webui")},
		{Kind: historyEntityVpsAdmin, ID: "api", Label: vpsAdminServiceLabel("api")},
		{Kind: historyEntityVpsAdmin, ID: "console", Label: vpsAdminServiceLabel("console")},
	}
}

func locationGroupTargets(loc *Location) []historyEntityInfo {
	if loc == nil {
		return nil
	}

	ret := make([]historyEntityInfo, 0, len(loc.NodeList)+len(loc.DnsResolverList))
	ret = append(ret, locationReportedTargets(loc)...)
	for _, resolver := range loc.DnsResolverList {
		ret = append(ret, historyEntityInfo{
			Kind:  historyEntityDnsResolver,
			ID:    resolver.Name,
			Label: resolver.Name,
		})
	}
	return ret
}

func locationReportedTargets(loc *Location) []historyEntityInfo {
	if loc == nil {
		return nil
	}

	ret := make([]historyEntityInfo, 0, len(loc.NodeList))
	for _, node := range loc.NodeList {
		ret = append(ret, historyEntityInfo{
			Kind:  historyEntityNode,
			ID:    node.Name,
			Label: node.Name,
		})
	}
	return ret
}

func servicesGroupTargets(services *Services) []historyEntityInfo {
	if services == nil {
		return nil
	}

	ret := make([]historyEntityInfo, 0, len(services.Web)+len(services.NameServer))
	for _, ws := range services.Web {
		ret = append(ret, historyEntityInfo{
			Kind:  historyEntityWebService,
			ID:    ws.Label,
			Label: ws.Label,
		})
	}
	for _, ns := range services.NameServer {
		ret = append(ret, historyEntityInfo{
			Kind:  historyEntityNameServer,
			ID:    ns.Name,
			Label: ns.Name,
		})
	}
	return ret
}

func groupAvailability(st *Status, probeTargets []historyEntityInfo, reportedTargets []historyEntityInfo, now time.Time) []availabilityResult {
	return groupAvailabilityWithData(st, probeTargets, reportedTargets, now, newHistoryData(st, now))
}

func groupAvailabilityWithData(st *Status, probeTargets []historyEntityInfo, reportedTargets []historyEntityInfo, now time.Time, data *historyData) []availabilityResult {
	data = ensureHistoryData(st, now, data)
	windows := availabilityWindows(now)
	ret := make([]availabilityResult, 0, len(windows))

	reports := data.reports
	mapping := data.mapping
	reportsAvailable := availabilityOutageReportsAvailable(st, reports)
	eventsByTarget := availabilityProbeEventsByTarget(st, probeTargets, windows)

	for _, window := range windows {
		result := availabilityResult{Label: window.Label}

		reportedMetrics := make([]availabilityMetric, 0, len(reportedTargets))
		for _, target := range reportedTargets {
			reportedMetrics = append(reportedMetrics, calculateReportedAvailability(
				target.Kind,
				target.ID,
				window,
				reports,
				mapping,
				reportsAvailable,
			))
		}
		result.Reported = averageAvailabilityMetrics(reportedMetrics)

		probeMetrics := make([]availabilityMetric, 0, len(probeTargets))
		for _, target := range probeTargets {
			if _, ok := availabilityProbeMethod(target.Kind); !ok {
				continue
			}

			events := eventsByTarget[historyKey(target.Kind, target.ID)]
			probeMetrics = append(probeMetrics, calculateProbeAvailability(window, events))
		}
		result.Probe = averageAvailabilityMetrics(probeMetrics)

		ret = append(ret, result)
	}

	return ret
}

func averageAvailabilityMetrics(metrics []availabilityMetric) availabilityMetric {
	total := 0.0
	count := 0
	for _, metric := range metrics {
		if !metric.Available {
			continue
		}
		total += metric.Percent
		count++
	}
	if count == 0 {
		return availabilityMetric{}
	}

	return availabilityMetric{
		Available: true,
		Percent:   roundAvailabilityPercent(total / float64(count)),
	}
}

func groupProbeEvents(st *Status, targets []historyEntityInfo, now time.Time) []ProbeEvent {
	if st == nil || st.History == nil {
		return nil
	}

	labels := make(map[string]string, len(targets))
	for _, target := range targets {
		labels[historyKey(target.Kind, target.ID)] = target.Label
	}
	ret := st.History.ProbeEventsForTargets(targets, now, historyDaysForStatus(st))

	for i := range ret {
		if ret[i].EntityLabel == "" {
			ret[i].EntityLabel = labels[historyKey(ret[i].EntityKind, ret[i].EntityID)]
		}
	}

	sort.SliceStable(ret, func(i, j int) bool {
		if ret[i].ChangedAt.Equal(ret[j].ChangedAt) {
			if ret[i].EntityLabel != ret[j].EntityLabel {
				return ret[i].EntityLabel < ret[j].EntityLabel
			}
			return ret[i].Method < ret[j].Method
		}
		return ret[i].ChangedAt.After(ret[j].ChangedAt)
	})

	return ret
}

func groupProbeLog(st *Status, targets []historyEntityInfo, now time.Time, limit int, offset int) ProbeLogPage {
	if st == nil || st.History == nil {
		return ProbeLogPage{}
	}

	ret := st.History.ProbeLogForTargets(targets, now, historyDaysForStatus(st), limit, offset)
	labels := make(map[string]string, len(targets))
	for _, target := range targets {
		labels[historyKey(target.Kind, target.ID)] = target.Label
	}
	for i := range ret.Events {
		if ret.Events[i].EntityLabel == "" {
			ret.Events[i].EntityLabel = labels[historyKey(ret.Events[i].EntityKind, ret.Events[i].EntityID)]
		}
	}
	return ret
}
