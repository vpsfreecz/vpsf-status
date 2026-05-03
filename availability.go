package main

import (
	"fmt"
	"math"
	"sort"
	"time"
)

const availabilityPrecision = 3

type availabilityWindow struct {
	Label string
	Start time.Time
	End   time.Time
}

type availabilityResult struct {
	Label    string
	Reported availabilityMetric
	Probe    availabilityMetric
}

type availabilityMetric struct {
	Available bool
	Percent   float64
}

type availabilityInterval struct {
	Start time.Time
	End   time.Time
}

type availabilityProbeSegment struct {
	availabilityInterval
	Status string
}

func availabilityWindows(now time.Time) []availabilityWindow {
	return []availabilityWindow{
		{Label: "30 days", Start: now.AddDate(0, 0, -30), End: now},
		{Label: "90 days", Start: now.AddDate(0, 0, -90), End: now},
		{Label: "180 days", Start: now.AddDate(0, 0, -180), End: now},
		{Label: "1 year", Start: now.AddDate(-1, 0, 0), End: now},
	}
}

func availabilityFetchStart(now time.Time, historyDays int) time.Time {
	windows := availabilityWindows(now)
	start := windows[len(windows)-1].Start
	historyStart := now.AddDate(0, 0, -historyWindowDays(historyDays))
	if historyStart.Before(start) {
		return historyStart
	}
	return start
}

func entityAvailability(st *Status, kind string, id string, now time.Time) []availabilityResult {
	return entityAvailabilityWithData(st, kind, id, now, newHistoryData(st, now))
}

func entityAvailabilityWithData(st *Status, kind string, id string, now time.Time, data *historyData) []availabilityResult {
	if _, ok := availabilityProbeMethod(kind); !ok {
		return nil
	}

	var reports []*OutageReport
	var mapping *historyEntityMapping
	reportsAvailable := false
	data = ensureHistoryData(st, now, data)
	if availabilityReportedOutageSupported(kind) {
		reports = data.reports
		mapping = data.mapping
		reportsAvailable = availabilityOutageReportsAvailable(st, reports)
	}
	windows := availabilityWindows(now)
	ret := make([]availabilityResult, 0, len(windows))
	eventsByTarget := availabilityProbeEventsByTarget(st, []historyEntityInfo{{Kind: kind, ID: id}}, windows)

	for _, window := range windows {
		events := eventsByTarget[historyKey(kind, id)]

		result := calculateAvailability(kind, id, window, events, reports, mapping, reportsAvailable)
		ret = append(ret, result)
	}

	return ret
}

func availabilityProbeEventsByTarget(st *Status, targets []historyEntityInfo, windows []availabilityWindow) map[string][]ProbeEvent {
	ret := make(map[string][]ProbeEvent)
	if st == nil || st.History == nil || len(targets) == 0 || len(windows) == 0 {
		return ret
	}

	start, end := availabilityWindowBounds(windows)
	return st.History.ProbeEventsForAvailabilityTargets(targets, start, end)
}

func availabilityWindowBounds(windows []availabilityWindow) (time.Time, time.Time) {
	if len(windows) == 0 {
		return time.Time{}, time.Time{}
	}

	start := windows[0].Start
	end := windows[0].End
	for _, window := range windows[1:] {
		if window.Start.Before(start) {
			start = window.Start
		}
		if window.End.After(end) {
			end = window.End
		}
	}
	return start, end
}

func calculateAvailability(
	kind string,
	id string,
	window availabilityWindow,
	events []ProbeEvent,
	reports []*OutageReport,
	mapping *historyEntityMapping,
	reportsAvailable bool,
) availabilityResult {
	return availabilityResult{
		Label:    window.Label,
		Reported: calculateReportedAvailability(kind, id, window, reports, mapping, reportsAvailable),
		Probe:    calculateProbeAvailability(window, events),
	}
}

func calculateReportedAvailability(
	kind string,
	id string,
	window availabilityWindow,
	reports []*OutageReport,
	mapping *historyEntityMapping,
	reportsAvailable bool,
) availabilityMetric {
	total := window.End.Sub(window.Start)
	if total <= 0 || !reportsAvailable || !availabilityReportedOutageSupported(kind) {
		return availabilityMetric{}
	}

	unavailable := outageUnavailableDuration(
		kind,
		id,
		reports,
		mapping,
		[]availabilityInterval{{Start: window.Start, End: window.End}},
		window.End,
	)

	return availabilityMetricFromUnavailable(total, unavailable)
}

func calculateProbeAvailability(window availabilityWindow, events []ProbeEvent) availabilityMetric {
	total := window.End.Sub(window.Start)
	if total <= 0 {
		return availabilityMetric{}
	}

	segments, missing := probeAvailabilitySegments(events, window.Start, window.End)
	if len(missing) > 0 {
		return availabilityMetric{}
	}

	unavailable := time.Duration(0)
	for _, segment := range segments {
		if !isAvailableProbeState(segment.Status) {
			unavailable += segment.End.Sub(segment.Start)
		}
	}

	return availabilityMetricFromUnavailable(total, unavailable)
}

func availabilityMetricFromUnavailable(total time.Duration, unavailable time.Duration) availabilityMetric {
	available := total - unavailable
	if available < 0 {
		available = 0
	}
	if available > total {
		available = total
	}

	return availabilityMetric{
		Available: true,
		Percent:   roundAvailabilityPercent(float64(available) / float64(total) * 100),
	}
}

func probeAvailabilitySegments(events []ProbeEvent, start time.Time, end time.Time) ([]availabilityProbeSegment, []availabilityInterval) {
	if !start.Before(end) {
		return nil, nil
	}

	events = availabilitySortedEvents(events, end)
	if len(events) == 0 {
		return nil, []availabilityInterval{{Start: start, End: end}}
	}

	var segments []availabilityProbeSegment
	var missing []availabilityInterval
	cursor := start
	currentStatus := ""
	eventIndex := 0

	for eventIndex < len(events) && !events[eventIndex].ChangedAt.After(start) {
		currentStatus = events[eventIndex].Status
		eventIndex++
	}

	if eventIndex == 0 {
		missing = appendInterval(missing, start, minTime(events[0].ChangedAt, end))
		cursor = minTime(events[0].ChangedAt, end)
		currentStatus = events[0].Status
		eventIndex = 1
	}

	for ; eventIndex < len(events); eventIndex++ {
		event := events[eventIndex]
		if event.ChangedAt.Before(start) || event.ChangedAt.After(end) {
			continue
		}

		segments = appendProbeSegment(segments, cursor, event.ChangedAt, currentStatus)
		cursor = event.ChangedAt
		currentStatus = event.Status
	}

	segments = appendProbeSegment(segments, cursor, end, currentStatus)
	return segments, missing
}

func availabilitySortedEvents(events []ProbeEvent, end time.Time) []ProbeEvent {
	ret := make([]ProbeEvent, 0, len(events))
	for _, event := range events {
		if event.ChangedAt.After(end) {
			continue
		}
		ret = append(ret, event)
	}

	sort.SliceStable(ret, func(i, j int) bool {
		if ret[i].ChangedAt.Equal(ret[j].ChangedAt) {
			return ret[i].Method < ret[j].Method
		}
		return ret[i].ChangedAt.Before(ret[j].ChangedAt)
	})
	return ret
}

func appendProbeSegment(segments []availabilityProbeSegment, start time.Time, end time.Time, status string) []availabilityProbeSegment {
	if !start.Before(end) {
		return segments
	}
	return append(segments, availabilityProbeSegment{
		availabilityInterval: availabilityInterval{Start: start, End: end},
		Status:               status,
	})
}

func appendInterval(intervals []availabilityInterval, start time.Time, end time.Time) []availabilityInterval {
	if !start.Before(end) {
		return intervals
	}
	return append(intervals, availabilityInterval{Start: start, End: end})
}

func outageUnavailableDuration(
	kind string,
	id string,
	reports []*OutageReport,
	mapping *historyEntityMapping,
	missing []availabilityInterval,
	now time.Time,
) time.Duration {
	if mapping == nil || len(reports) == 0 || len(missing) == 0 {
		return 0
	}

	key := historyKey(kind, id)
	unavailable := make([]availabilityInterval, 0)
	for _, report := range reports {
		if report == nil || report.BeginsAt.IsZero() {
			continue
		}
		if _, ok := mapping.outageHistoryKeys(report)[key]; !ok {
			continue
		}

		reportStart := report.BeginsAt
		reportEnd := outageEndsAt(report)
		if reportEnd.IsZero() || !reportEnd.After(reportStart) {
			if report.State == "announced" && reportStart.Before(now) {
				reportEnd = now
			} else {
				continue
			}
		}

		for _, interval := range missing {
			start := maxTime(reportStart, interval.Start)
			end := minTime(reportEnd, interval.End)
			unavailable = appendInterval(unavailable, start, end)
		}
	}

	return mergedIntervalDuration(unavailable)
}

func mergedIntervalDuration(intervals []availabilityInterval) time.Duration {
	if len(intervals) == 0 {
		return 0
	}

	sort.Slice(intervals, func(i, j int) bool {
		if intervals[i].Start.Equal(intervals[j].Start) {
			return intervals[i].End.Before(intervals[j].End)
		}
		return intervals[i].Start.Before(intervals[j].Start)
	})

	total := time.Duration(0)
	current := intervals[0]
	for _, interval := range intervals[1:] {
		if interval.Start.After(current.End) {
			total += current.End.Sub(current.Start)
			current = interval
			continue
		}
		if interval.End.After(current.End) {
			current.End = interval.End
		}
	}
	total += current.End.Sub(current.Start)
	return total
}

func availabilityProbeMethod(kind string) (string, bool) {
	switch kind {
	case historyEntityNode:
		return "Ping", true
	case historyEntityDnsResolver, historyEntityNameServer:
		return "Lookup", true
	case historyEntityWebService, historyEntityVpsAdmin:
		return "HTTP", true
	default:
		return "", false
	}
}

func availabilityReportedOutageSupported(kind string) bool {
	return kind == historyEntityNode || kind == historyEntityVpsAdmin
}

func availabilityOutageReportsAvailable(st *Status, reports []*OutageReport) bool {
	if st == nil {
		return false
	}
	if st.OutageReports != nil && st.OutageReports.Status {
		return true
	}
	return len(reports) > 0
}

func isAvailableProbeState(status string) bool {
	return status == "" || status == historyProbeStateOperational || status == historyProbeStateDegraded
}

func roundAvailabilityPercent(percent float64) float64 {
	scale := math.Pow10(availabilityPrecision)
	return math.Round(percent*scale) / scale
}

func formatAvailabilityPercent(percent float64) string {
	return fmt.Sprintf("%.*f%%", availabilityPrecision, percent)
}

func minTime(a time.Time, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func maxTime(a time.Time, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
