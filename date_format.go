package main

import (
	"fmt"
	"time"
)

func formatGeneratedAt(t time.Time, loc *pageLocale) string {
	if loc != nil && loc.codeOrDefault() == "cs" {
		return formatCzechDateTime(t)
	}
	return t.Format(time.UnixDate)
}

func formatNoticeTimestamp(t time.Time, loc *pageLocale) string {
	if loc != nil && loc.codeOrDefault() == "cs" {
		return formatCzechDateTime(t)
	}
	return t.Local().Format("Mon Jan _2 15:04:05 MST 2006")
}

func formatHistoryDay(t time.Time, loc *pageLocale) string {
	if loc != nil && loc.codeOrDefault() == "cs" {
		local := t.Local()
		return fmt.Sprintf("%d. %d. %04d", local.Day(), int(local.Month()), local.Year())
	}
	return t.Format("Jan _2, 2006")
}

func formatCzechDateTime(t time.Time) string {
	local := t.Local()
	zone, _ := local.Zone()
	return fmt.Sprintf(
		"%d. %d. %04d %02d:%02d:%02d %s",
		local.Day(),
		int(local.Month()),
		local.Year(),
		local.Hour(),
		local.Minute(),
		local.Second(),
		zone,
	)
}
