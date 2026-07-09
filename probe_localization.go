package main

import (
	"regexp"
	"strings"

	"github.com/vpsfreecz/vpsf-status/internal/i18n/catalog"
)

var probePacketLossRe = regexp.MustCompile(`^([0-9]+(?:\.[0-9]+)?)%\s+packet loss$`)
var (
	storageScrubRe    = regexp.MustCompile(`^storage is being scrubbed to check data integrity,\s+([0-9]+(?:\.[0-9]+)?)\s+%\s+done$`)
	storageResilverRe = regexp.MustCompile(`^storage is being resilvered to replace disks,\s+([0-9]+(?:\.[0-9]+)?)\s+%\s+done$`)
)

func probeMethodLabelForLocale(method string, loc *pageLocale) string {
	if loc == nil {
		loc = defaultPageLocale()
	}

	switch normalizedProbeText(method) {
	case "http":
		return loc.T(catalog.MsgProbeMethodHTTP)
	case "ping":
		return loc.T(catalog.MsgProbeMethodPing)
	case "lookup", "dns", "dns lookup":
		return loc.T(catalog.MsgProbeMethodLookup)
	case "storage":
		return loc.T(catalog.MsgProbeMethodStorage)
	case "vpsadmin":
		return loc.T(catalog.MsgProbeMethodVpsAdmin)
	default:
		return method
	}
}

func probeMessageForLocale(message string, loc *pageLocale) string {
	if loc == nil {
		loc = defaultPageLocale()
	}

	id, data, ok := probeMessageCatalogEntry(message)
	if !ok {
		return message
	}
	if data == nil {
		return loc.T(id)
	}
	return loc.TD(id, data)
}

func probeStatusDescriptionForLocale(method string, message string, loc *pageLocale) string {
	methodLabel := probeMethodLabelForLocale(method, loc)
	messageLabel := probeMessageForLocale(message, loc)

	if isLookupProbeMethod(method) {
		if loc != nil && loc.codeOrDefault() == "cs" {
			if _, ok := lookupProbeMessageSuffix(message); ok {
				return messageLabel
			}
		} else if suffix, ok := lookupProbeMessageSuffix(message); ok {
			return methodLabel + " " + suffix
		}
	}

	if methodLabel == "" {
		return messageLabel
	}
	if messageLabel == "" {
		return methodLabel
	}
	if probeMessageIncludesMethod(method, methodLabel, message, messageLabel, loc) {
		return messageLabel
	}
	return methodLabel + " " + messageLabel
}

func probeEntityLabelForLocale(kind string, id string, storedLabel string, loc *pageLocale) string {
	if loc == nil {
		loc = defaultPageLocale()
	}

	if kind == historyEntityVpsAdmin {
		switch id {
		case "api", "webui", "console":
			return vpsAdminServiceLabelForLocale(id, loc)
		}
		if loc.codeOrDefault() == "cs" && normalizedProbeText(storedLabel) == "remote console" {
			return loc.T(catalog.MsgServiceVpsAdminConsole)
		}
	}

	if storedLabel != "" {
		return storedLabel
	}
	return id
}

func probeMessageCatalogEntry(message string) (string, map[string]any, bool) {
	trimmed := strings.TrimSpace(message)
	switch normalizedProbeText(trimmed) {
	case "check failed":
		return catalog.MsgProbeMessageCheckFailed, nil, true
	case "lookup failed":
		return catalog.MsgProbeMessageLookupFailed, nil, true
	case "lookup succeeded":
		return catalog.MsgProbeMessageLookupSucceeded, nil, true
	case "not reporting":
		return catalog.MsgProbeMessageNotReporting, nil, true
	case "under maintenance":
		return catalog.MsgProbeMessageUnderMaint, nil, true
	case "reporting":
		return catalog.MsgProbeMessageReporting, nil, true
	case "responding":
		return catalog.MsgProbeMessageResponding, nil, true
	case "not responding":
		return catalog.MsgProbeMessageNotResponding, nil, true
	case "storage status check failed":
		return catalog.MsgStorageCheckFailed, nil, true
	case "storage scan status check failed":
		return catalog.MsgStorageScanFailed, nil, true
	case "unable to determine storage status":
		return catalog.MsgStorageUnableStatus, nil, true
	case "storage is online":
		return catalog.MsgStorageOnline, nil, true
	case "one or more disks have failed, storage continues to function":
		return catalog.MsgStorageDegraded, nil, true
	case "storage not operational":
		return catalog.MsgStorageNotOperational, nil, true
	case "storage is in an unknown state", "storage is in a unknown state":
		return catalog.MsgStorageUnknownState, nil, true
	case "not running":
		return catalog.MsgStorageScanNone, nil, true
	case "unable to determine storage scan status":
		return catalog.MsgStorageScanUnable, nil, true
	case "storage scan is in an unknown state", "storage scan is in a unknown state":
		return catalog.MsgStorageScanUnknown, nil, true
	}

	if match := probePacketLossRe.FindStringSubmatch(strings.ToLower(trimmed)); match != nil {
		return catalog.MsgProbeMessagePacketLoss, map[string]any{"Percent": match[1]}, true
	}
	if match := storageScrubRe.FindStringSubmatch(strings.ToLower(trimmed)); match != nil {
		return catalog.MsgStorageScanScrub, map[string]any{"Percent": match[1]}, true
	}
	if match := storageResilverRe.FindStringSubmatch(strings.ToLower(trimmed)); match != nil {
		return catalog.MsgStorageScanResilver, map[string]any{"Percent": match[1]}, true
	}

	return "", nil, false
}

func probeMessageIncludesMethod(method string, methodLabel string, message string, messageLabel string, loc *pageLocale) bool {
	methodNorm := normalizedProbeText(method)
	methodLabelNorm := normalizedProbeText(methodLabel)
	messageNorm := normalizedProbeText(message)
	messageLabelNorm := normalizedProbeText(messageLabel)

	if methodNorm != "" && (strings.HasPrefix(messageNorm, methodNorm+" ") || messageNorm == methodNorm) {
		return true
	}
	if methodLabelNorm != "" && (strings.HasPrefix(messageLabelNorm, methodLabelNorm+" ") || messageLabelNorm == methodLabelNorm) {
		return true
	}
	if methodNorm == "http" && strings.HasPrefix(messageNorm, "http ") {
		return true
	}
	if methodNorm == "storage" && strings.HasPrefix(messageNorm, "storage ") {
		return true
	}
	if isLookupProbeMethod(method) && loc != nil && loc.codeOrDefault() == "cs" {
		_, _, ok := probeMessageCatalogEntry(message)
		return ok
	}
	return false
}

func lookupProbeMessageSuffix(message string) (string, bool) {
	switch normalizedProbeText(message) {
	case "lookup failed":
		return "failed", true
	case "lookup succeeded":
		return "succeeded", true
	default:
		return "", false
	}
}

func isLookupProbeMethod(method string) bool {
	switch normalizedProbeText(method) {
	case "lookup", "dns", "dns lookup":
		return true
	default:
		return false
	}
}

func normalizedProbeText(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(s))), " ")
}
