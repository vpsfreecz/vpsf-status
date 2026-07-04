package main

import (
	"sort"
	"strconv"
	"time"
)

func createGroupDetailView(st *Status, kind string, id string, now time.Time, probePage int) (EntityDetailView, bool) {
	return createGroupDetailViewForLocale(st, kind, id, now, probePage, defaultPageLocale())
}

func createGroupDetailViewForLocale(st *Status, kind string, id string, now time.Time, probePage int, loc *pageLocale) (EntityDetailView, bool) {
	if st == nil || kind == "" {
		return EntityDetailView{}, false
	}

	ret := EntityDetailView{
		Lang:            loc.codeOrDefault(),
		Kind:            kind,
		ID:              id,
		Group:           loc.T("entity.group.group"),
		ShowEventEntity: true,
	}

	var groupTarget historyGroupTarget
	var probeTargets []historyEntityInfo
	var reportedTargets []historyEntityInfo

	switch kind {
	case historyGroupVpsAdmin:
		view := createVpsAdminView(st.VpsAdmin)
		ret.Label = "vpsAdmin"
		ret.StatusText, ret.StatusClass = groupStatusText(view.IsOperational(), view.IsDegraded(), loc)
		ret.ShowReportedAvailability = true
		groupTarget = historyGroupTarget{Kind: historyGroupVpsAdmin}
		probeTargets = vpsAdminGroupTargetsForLocale(loc)
		reportedTargets = probeTargets
	case historyGroupLocation:
		locID, err := strconv.Atoi(id)
		if err != nil {
			return EntityDetailView{}, false
		}
		location := findLocationByID(st, locID)
		if location == nil {
			return EntityDetailView{}, false
		}

		view := createLocationView([]*Location{location})[0]
		ret.Label = location.Label
		ret.Group = loc.T("entity.group.location")
		ret.StatusText, ret.StatusClass = groupStatusText(view.IsOperational(), view.IsDegraded(), loc)
		ret.ShowReportedAvailability = true
		groupTarget = historyGroupTarget{Kind: historyGroupLocation, LocationID: location.Id}
		probeTargets = locationGroupTargets(location)
		reportedTargets = locationReportedTargets(location)
	case historyGroupServices:
		view := createServicesView(st.Services)
		ret.Label = loc.T("section.services")
		ret.StatusText, ret.StatusClass = groupStatusText(view.IsOperational(), view.IsDegraded(), loc)
		groupTarget = historyGroupTarget{Kind: historyGroupServices}
		probeTargets = servicesGroupTargets(st.Services)
	default:
		return EntityDetailView{}, false
	}

	data := newHistoryData(st, now)
	ret.History = createGroupHistoryViewForLocale(st, now, groupTarget, ret.Label, data, loc)
	ret.Availability = availabilityDetailViews(groupAvailabilityWithDataForLocale(st, probeTargets, reportedTargets, now, data, loc), loc)
	logPage, page := paginatedProbeLog(probePage, func(limit int, offset int) ProbeLogPage {
		return groupProbeLog(st, probeTargets, now, limit, offset)
	})
	ret.Events = probeLogEventDetailViewsForLocale(st, logPage.Events, now, data, loc)
	ret.EventPagination = newProbeLogPaginationView("/group", kind, id, page, logPage.Total, loc.codeOrDefault())
	return ret, true
}

func groupStatusText(operational bool, degraded bool, loc *pageLocale) (string, string) {
	if operational {
		return loc.T("status.operational"), "success"
	}
	if degraded {
		return loc.T("status.degraded"), "warning"
	}
	return loc.T("status.down"), "danger"
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
	return vpsAdminGroupTargetsForLocale(defaultPageLocale())
}

func vpsAdminGroupTargetsForLocale(loc *pageLocale) []historyEntityInfo {
	return []historyEntityInfo{
		{Kind: historyEntityVpsAdmin, ID: "webui", Label: vpsAdminServiceLabelForLocale("webui", loc)},
		{Kind: historyEntityVpsAdmin, ID: "api", Label: vpsAdminServiceLabelForLocale("api", loc)},
		{Kind: historyEntityVpsAdmin, ID: "console", Label: vpsAdminServiceLabelForLocale("console", loc)},
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
	return groupAvailabilityWithDataForLocale(st, probeTargets, reportedTargets, now, newHistoryData(st, now), defaultPageLocale())
}

func groupAvailabilityWithData(st *Status, probeTargets []historyEntityInfo, reportedTargets []historyEntityInfo, now time.Time, data *historyData) []availabilityResult {
	return groupAvailabilityWithDataForLocale(st, probeTargets, reportedTargets, now, data, defaultPageLocale())
}

func groupAvailabilityWithDataForLocale(st *Status, probeTargets []historyEntityInfo, reportedTargets []historyEntityInfo, now time.Time, data *historyData, loc *pageLocale) []availabilityResult {
	data = ensureHistoryData(st, now, data)
	windows := availabilityWindowsForLocale(now, loc)
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
