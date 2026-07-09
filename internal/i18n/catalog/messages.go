package catalog

import "github.com/nicksnyder/go-i18n/v2/i18n"

const (
	MsgAppTitle = "app.title"

	MsgLanguageLabel    = "language.label"
	MsgLanguageEnglish  = "language.en"
	MsgLanguageCzech    = "language.cs"
	MsgLanguageSwitchTo = "language.switch_to"

	MsgNavStatus       = "nav.status"
	MsgNavBackToStatus = "nav.back_to_status"
	MsgNavRenderedAt   = "nav.rendered_at"

	MsgFooterPrometheusMetrics = "footer.prometheus_metrics"
	MsgFooterAbout             = "footer.about"

	MsgNoticeUpdatedAt = "notice.updated_at"

	MsgLoadingTitle = "loading.title"
	MsgLoadingBody  = "loading.body"

	MsgStatusCountsAria       = "status_counts.aria"
	MsgStatusOperational      = "status.operational"
	MsgStatusDegraded         = "status.degraded"
	MsgStatusDegradedMaint    = "status.degraded_or_maintenance"
	MsgStatusDown             = "status.down"
	MsgStatusUnderMaintenance = "status.under_maintenance"
	MsgStatusOnline           = "status.online"
	MsgStatusResponding       = "status.responding"
	MsgStatusError            = "status.error"
	MsgStatusUnknown          = "status.unknown"
	MsgStatusTotal            = "status.total"
	MsgStatusNotSupported     = "status.not_supported"
	MsgStatusNotAvailable     = "status.not_available"
	MsgStatusProbeMaintenance = "status.probe.maintenance"
	MsgStatusProbeOperational = "status.probe.operational"
	MsgStatusProbeDegraded    = "status.probe.degraded"
	MsgStatusProbeDown        = "status.probe.down"
	MsgStatusProbeError       = "status.probe.error"

	MsgOutagesReportedPlanned             = "outages.reported.planned"
	MsgOutagesReportedUnplanned           = "outages.reported.unplanned"
	MsgOutagesReportedPlannedAndUnplanned = "outages.reported.planned_and_unplanned"
	MsgOutagesRecentPlanned               = "outages.recent.planned"
	MsgOutagesRecentUnplanned             = "outages.recent.unplanned"
	MsgOutagesRecentPlannedAndUnplanned   = "outages.recent.planned_and_unplanned"
	MsgOutagesNoIssues                    = "outages.no_issues"
	MsgOutagesHistory                     = "outages.history"
	MsgOutagesUnableFetch                 = "outages.unable_fetch"
	MsgOutageTypePlanned                  = "outage.type.planned"
	MsgOutageTypeUnplanned                = "outage.type.unplanned"
	MsgOutageSummaryFallback              = "outage.summary.fallback"
	MsgOutageSummaryPlanned               = "outage.summary.planned"
	MsgOutageSummaryUnplanned             = "outage.summary.unplanned"
	MsgOutageImpactTBD                    = "outage.impact.tbd"
	MsgOutageImpactSystemRestart          = "outage.impact.system_restart"
	MsgOutageImpactSystemReset            = "outage.impact.system_reset"
	MsgOutageImpactNetwork                = "outage.impact.network"
	MsgOutageImpactPerformance            = "outage.impact.performance"
	MsgOutageImpactUnavailability         = "outage.impact.unavailability"
	MsgOutageImpactExport                 = "outage.impact.export"
	MsgOutageMoreInformation              = "outage.more_information"

	MsgSecurityRecent      = "security.recent"
	MsgSecurityUnableFetch = "security.unable_fetch"

	MsgTableDate        = "table.date"
	MsgTableDuration    = "table.duration"
	MsgTableType        = "table.type"
	MsgTableSystems     = "table.systems"
	MsgTableImpact      = "table.impact"
	MsgTableReason      = "table.reason"
	MsgTableLink        = "table.link"
	MsgTablePublished   = "table.published"
	MsgTableCVE         = "table.cve"
	MsgTableSummary     = "table.summary"
	MsgTableService     = "table.service"
	MsgTableDescription = "table.description"
	MsgTableStatus      = "table.status"
	MsgTableNode        = "table.node"
	MsgTableStorage     = "table.storage"
	MsgTablePing        = "table.ping"
	MsgTableName        = "table.name"
	MsgTableLookup      = "table.lookup"
	MsgTableTime        = "table.time"
	MsgTableEntity      = "table.entity"
	MsgTableMethod      = "table.method"
	MsgTableMessage     = "table.message"
	MsgTableCoveredBy   = "table.covered_by"
	MsgTablePeriod      = "table.period"
	MsgTableReported    = "table.reported"
	MsgTableProbe       = "table.probe"

	MsgSectionWebServices  = "section.web_services"
	MsgSectionNodes        = "section.nodes"
	MsgSectionDNS          = "section.dns_resolvers"
	MsgSectionServices     = "section.services"
	MsgSectionNameServers  = "section.name_servers"
	MsgSectionAvailability = "section.availability"
	MsgSectionProbeLog     = "section.probe_log"

	MsgServiceVpsAdminWebuiDesc   = "service.vpsadmin.webui.description"
	MsgServiceVpsAdminApiDesc     = "service.vpsadmin.api.description"
	MsgServiceVpsAdminConsole     = "service.vpsadmin.console"
	MsgServiceVpsAdminConsoleDesc = "service.vpsadmin.console.description"

	MsgEntityGroupNode        = "entity.group.node"
	MsgEntityGroupVpsAdmin    = "entity.group.vpsadmin"
	MsgEntityGroupDNSResolver = "entity.group.dns_resolver"
	MsgEntityGroupWebService  = "entity.group.web_service"
	MsgEntityGroupNameServer  = "entity.group.name_server"
	MsgEntityGroupGroup       = "entity.group.group"
	MsgEntityGroupLocation    = "entity.group.location"

	MsgAvailability30Days  = "availability.30_days"
	MsgAvailability90Days  = "availability.90_days"
	MsgAvailability180Days = "availability.180_days"
	MsgAvailability1Year   = "availability.1_year"

	MsgProbeLogPages     = "probe_log.pages"
	MsgProbeLogPrevious  = "probe_log.previous"
	MsgProbeLogNext      = "probe_log.next"
	MsgProbeLogNoChanges = "probe_log.no_changes"

	MsgProbeMethodHTTP     = "probe.method.http"
	MsgProbeMethodPing     = "probe.method.ping"
	MsgProbeMethodLookup   = "probe.method.lookup"
	MsgProbeMethodStorage  = "probe.method.storage"
	MsgProbeMethodVpsAdmin = "probe.method.vpsadmin"

	MsgProbeMessageCheckFailed     = "probe.message.check_failed"
	MsgProbeMessageLookupFailed    = "probe.message.lookup_failed"
	MsgProbeMessageLookupSucceeded = "probe.message.lookup_succeeded"
	MsgProbeMessageNotReporting    = "probe.message.not_reporting"
	MsgProbeMessageUnderMaint      = "probe.message.under_maintenance"
	MsgProbeMessageReporting       = "probe.message.reporting"
	MsgProbeMessageResponding      = "probe.message.responding"
	MsgProbeMessageNotResponding   = "probe.message.not_responding"
	MsgProbeMessagePacketLoss      = "probe.message.packet_loss"

	MsgHistoryAria                 = "history.aria"
	MsgHistoryNoIncidents          = "history.no_incidents"
	MsgHistoryDaySummaryEmpty      = "history.day_summary.empty"
	MsgHistoryDaySummaryIncidents  = "history.day_summary.incidents"
	MsgHistoryRemoved              = "history.removed"
	MsgHistoryIncidentLane         = "history.incident.lane"
	MsgHistoryIncidentStarted      = "history.incident.started"
	MsgHistoryIncidentExpected     = "history.incident.expected_duration"
	MsgHistoryIncidentObserved     = "history.incident.observed_duration"
	MsgHistoryIncidentDuration     = "history.incident.duration"
	MsgHistoryIncidentDurationOpen = "history.incident.duration_open"
	MsgHistoryProbeIncident        = "history.probe_incident"

	MsgStorageUnableStatus   = "storage.unable_status"
	MsgStorageOnline         = "storage.online"
	MsgStorageDegraded       = "storage.degraded"
	MsgStorageNotOperational = "storage.not_operational"
	MsgStorageCheckFailed    = "storage.check_failed"
	MsgStorageUnknownState   = "storage.unknown_state"
	MsgStorageScanNone       = "storage.scan.none"
	MsgStorageScanScrub      = "storage.scan.scrub"
	MsgStorageScanResilver   = "storage.scan.resilver"
	MsgStorageScanUnable     = "storage.scan.unable"
	MsgStorageScanFailed     = "storage.scan.failed"
	MsgStorageScanUnknown    = "storage.scan.unknown"

	MsgAboutTitle      = "about.title"
	MsgAboutIntro      = "about.intro"
	MsgAboutChecks     = "about.checks"
	MsgAboutReportsA   = "about.reports.before_vpsadmin"
	MsgAboutReportsB   = "about.reports.before_history"
	MsgAboutHistory    = "about.history"
	MsgAboutReportsC   = "about.reports.after_history"
	MsgAboutSourceA    = "about.source.before_link"
	MsgAboutSourceLink = "about.source.link"
)

func Messages() []*i18n.Message {
	messages := []*i18n.Message{
		{ID: MsgAppTitle, Other: "vpsFree.cz Status"},
		{ID: MsgLanguageLabel, Other: "Language"},
		{ID: MsgLanguageEnglish, Other: "English"},
		{ID: MsgLanguageCzech, Other: "Czech"},
		{ID: MsgLanguageSwitchTo, Other: "Switch to {{.Language}}"},
		{ID: MsgNavStatus, Other: "Status"},
		{ID: MsgNavBackToStatus, Other: "Back to Status"},
		{ID: MsgNavRenderedAt, Other: "Rendered at:"},
		{ID: MsgFooterPrometheusMetrics, Other: "Prometheus metrics"},
		{ID: MsgFooterAbout, Other: "About"},
		{ID: MsgNoticeUpdatedAt, Other: "Updated at {{.UpdatedAt}}"},
		{ID: MsgLoadingTitle, Other: "Initializing..."},
		{ID: MsgLoadingBody, Other: "vpsFree.cz Status is initializing and should be ready in a few seconds. Please refresh the page."},
		{ID: MsgStatusCountsAria, Other: "{{.Operational}} operational, {{.Degraded}} degraded or under maintenance, {{.Down}} down, {{.Total}} total"},
		{ID: MsgStatusOperational, Other: "Operational"},
		{ID: MsgStatusDegraded, Other: "Degraded"},
		{ID: MsgStatusDegradedMaint, Other: "Degraded or under maintenance"},
		{ID: MsgStatusDown, Other: "Down"},
		{ID: MsgStatusUnderMaintenance, Other: "Under maintenance"},
		{ID: MsgStatusOnline, Other: "Online"},
		{ID: MsgStatusResponding, Other: "Responding"},
		{ID: MsgStatusError, Other: "Error"},
		{ID: MsgStatusUnknown, Other: "Unknown"},
		{ID: MsgStatusTotal, Other: "Total"},
		{ID: MsgStatusNotSupported, Other: "Not supported"},
		{ID: MsgStatusNotAvailable, Other: "n/a"},
		{ID: MsgStatusProbeMaintenance, Other: "Maintenance"},
		{ID: MsgStatusProbeOperational, Other: "Operational"},
		{ID: MsgStatusProbeDegraded, Other: "Degraded"},
		{ID: MsgStatusProbeDown, Other: "Down"},
		{ID: MsgStatusProbeError, Other: "Error"},
		{ID: MsgOutagesReportedPlanned, Other: "Reported maintenance"},
		{ID: MsgOutagesReportedUnplanned, Other: "Reported outages"},
		{ID: MsgOutagesReportedPlannedAndUnplanned, Other: "Reported maintenance and outages"},
		{ID: MsgOutagesRecentPlanned, Other: "Recent maintenance"},
		{ID: MsgOutagesRecentUnplanned, Other: "Recent outages"},
		{ID: MsgOutagesRecentPlannedAndUnplanned, Other: "Recent maintenance and outages"},
		{ID: MsgOutagesNoIssues, Other: "No issues reported. See"},
		{ID: MsgOutagesHistory, Other: "history"},
		{ID: MsgOutagesUnableFetch, Other: "Unable to fetch outage reports from vpsAdmin."},
		{ID: MsgOutageTypePlanned, Other: "Planned outage"},
		{ID: MsgOutageTypeUnplanned, Other: "Unplanned outage"},
		{ID: MsgOutageSummaryFallback, Other: "Outage #{{.ID}}"},
		{ID: MsgOutageSummaryPlanned, Other: "Planned outage: {{.Summary}}"},
		{ID: MsgOutageSummaryUnplanned, Other: "Unplanned outage: {{.Summary}}"},
		{ID: MsgOutageImpactTBD, Other: "TBD"},
		{ID: MsgOutageImpactSystemRestart, Other: "System restart"},
		{ID: MsgOutageImpactSystemReset, Other: "System reset"},
		{ID: MsgOutageImpactNetwork, Other: "Network"},
		{ID: MsgOutageImpactPerformance, Other: "Performance"},
		{ID: MsgOutageImpactUnavailability, Other: "Unavailability"},
		{ID: MsgOutageImpactExport, Other: "NFS export"},
		{ID: MsgOutageMoreInformation, Other: "More information"},
		{ID: MsgSecurityRecent, Other: "Recent security advisories"},
		{ID: MsgSecurityUnableFetch, Other: "Unable to fetch security advisories from vpsAdmin."},
		{ID: MsgTableDate, Other: "Date"},
		{ID: MsgTableDuration, Other: "Duration"},
		{ID: MsgTableType, Other: "Type"},
		{ID: MsgTableSystems, Other: "Systems"},
		{ID: MsgTableImpact, Other: "Impact"},
		{ID: MsgTableReason, Other: "Reason"},
		{ID: MsgTableLink, Other: "Link"},
		{ID: MsgTablePublished, Other: "Published"},
		{ID: MsgTableCVE, Other: "CVE"},
		{ID: MsgTableSummary, Other: "Summary"},
		{ID: MsgTableService, Other: "Service"},
		{ID: MsgTableDescription, Other: "Description"},
		{ID: MsgTableStatus, Other: "Status"},
		{ID: MsgTableNode, Other: "Node"},
		{ID: MsgTableStorage, Other: "Storage"},
		{ID: MsgTablePing, Other: "Ping"},
		{ID: MsgTableName, Other: "Name"},
		{ID: MsgTableLookup, Other: "Lookup"},
		{ID: MsgTableTime, Other: "Time"},
		{ID: MsgTableEntity, Other: "Entity"},
		{ID: MsgTableMethod, Other: "Method"},
		{ID: MsgTableMessage, Other: "Message"},
		{ID: MsgTableCoveredBy, Other: "Covered by"},
		{ID: MsgTablePeriod, Other: "Period"},
		{ID: MsgTableReported, Other: "Reported"},
		{ID: MsgTableProbe, Other: "Probe"},
		{ID: MsgSectionWebServices, Other: "Web Services"},
		{ID: MsgSectionNodes, Other: "Nodes"},
		{ID: MsgSectionDNS, Other: "DNS Resolvers"},
		{ID: MsgSectionServices, Other: "Services"},
		{ID: MsgSectionNameServers, Other: "Name Servers"},
		{ID: MsgSectionAvailability, Other: "Availability"},
		{ID: MsgSectionProbeLog, Other: "Probe log"},
		{ID: MsgServiceVpsAdminWebuiDesc, Other: "Main web interface for vpsAdmin"},
		{ID: MsgServiceVpsAdminApiDesc, Other: "HTTP API server"},
		{ID: MsgServiceVpsAdminConsole, Other: "Remote Console"},
		{ID: MsgServiceVpsAdminConsoleDesc, Other: "Interface for VPS remote console"},
		{ID: MsgEntityGroupNode, Other: "Node"},
		{ID: MsgEntityGroupVpsAdmin, Other: "vpsAdmin"},
		{ID: MsgEntityGroupDNSResolver, Other: "DNS Resolver"},
		{ID: MsgEntityGroupWebService, Other: "Web Service"},
		{ID: MsgEntityGroupNameServer, Other: "Name Server"},
		{ID: MsgEntityGroupGroup, Other: "Group"},
		{ID: MsgEntityGroupLocation, Other: "Location"},
		{ID: MsgAvailability30Days, Other: "30 days"},
		{ID: MsgAvailability90Days, Other: "90 days"},
		{ID: MsgAvailability180Days, Other: "180 days"},
		{ID: MsgAvailability1Year, Other: "1 year"},
		{ID: MsgProbeLogPages, Other: "Probe log pages"},
		{ID: MsgProbeLogPrevious, Other: "Previous"},
		{ID: MsgProbeLogNext, Other: "Next"},
		{ID: MsgProbeLogNoChanges, Other: "No probe changes recorded."},
		{ID: MsgProbeMethodHTTP, Other: "HTTP"},
		{ID: MsgProbeMethodPing, Other: "Ping"},
		{ID: MsgProbeMethodLookup, Other: "DNS lookup"},
		{ID: MsgProbeMethodStorage, Other: "Storage"},
		{ID: MsgProbeMethodVpsAdmin, Other: "vpsAdmin"},
		{ID: MsgProbeMessageCheckFailed, Other: "check failed"},
		{ID: MsgProbeMessageLookupFailed, Other: "lookup failed"},
		{ID: MsgProbeMessageLookupSucceeded, Other: "lookup succeeded"},
		{ID: MsgProbeMessageNotReporting, Other: "not reporting"},
		{ID: MsgProbeMessageUnderMaint, Other: "under maintenance"},
		{ID: MsgProbeMessageReporting, Other: "reporting"},
		{ID: MsgProbeMessageResponding, Other: "responding"},
		{ID: MsgProbeMessageNotResponding, Other: "not responding"},
		{ID: MsgProbeMessagePacketLoss, Other: "{{.Percent}}% packet loss"},
		{ID: MsgHistoryAria, Other: "{{.Label}} history"},
		{ID: MsgHistoryNoIncidents, Other: "No incidents"},
		{ID: MsgHistoryDaySummaryEmpty, Other: "{{.Date}}: no incidents"},
		{ID: MsgHistoryDaySummaryIncidents, Other: "{{.Date}}: {{.Summary}}"},
		{ID: MsgHistoryRemoved, Other: "{{.Label}} (removed)"},
		{ID: MsgHistoryIncidentLane, Other: "{{.Label}}: {{.Text}}"},
		{ID: MsgHistoryIncidentStarted, Other: "Started: {{.Time}}"},
		{ID: MsgHistoryIncidentExpected, Other: "Expected duration"},
		{ID: MsgHistoryIncidentObserved, Other: "Observed duration"},
		{ID: MsgHistoryIncidentDuration, Other: "{{.Label}}: {{.Minutes}} min"},
		{ID: MsgHistoryIncidentDurationOpen, Other: "{{.Label}}: {{.Minutes}} min so far"},
		{ID: MsgHistoryProbeIncident, Other: "{{.Label}}: {{.Message}}"},
		{ID: MsgStorageUnableStatus, Other: "Unable to determine storage status"},
		{ID: MsgStorageOnline, Other: "Storage is online"},
		{ID: MsgStorageDegraded, Other: "One or more disks have failed, storage continues to function"},
		{ID: MsgStorageNotOperational, Other: "Storage not operational"},
		{ID: MsgStorageCheckFailed, Other: "Storage status check failed"},
		{ID: MsgStorageUnknownState, Other: "Storage is in an unknown state"},
		{ID: MsgStorageScanNone, Other: "Not running"},
		{ID: MsgStorageScanScrub, Other: "Storage is being scrubbed to check data integrity, {{.Percent}} % done"},
		{ID: MsgStorageScanResilver, Other: "Storage is being resilvered to replace disks, {{.Percent}} % done"},
		{ID: MsgStorageScanUnable, Other: "Unable to determine storage scan status"},
		{ID: MsgStorageScanFailed, Other: "Storage scan status check failed"},
		{ID: MsgStorageScanUnknown, Other: "Storage scan is in an unknown state"},
		{ID: MsgAboutTitle, Other: "About vpsFree.cz Status"},
		{ID: MsgAboutIntro, Other: "is an application that monitors the state of vpsFree.cz's infrastructure. It uses an independent network connection and should thus be available in case of issues with our primary connection or other parts of our infrastructure."},
		{ID: MsgAboutChecks, Other: "The checks are run automatically every {{.Interval}} seconds."},
		{ID: MsgAboutReportsA, Other: "Verified issues that impact our members are reported through"},
		{ID: MsgAboutReportsB, Other: "when possible. See the"},
		{ID: MsgAboutHistory, Other: "history"},
		{ID: MsgAboutReportsC, Other: "of such planned and unplanned outages. In case of an outage when vpsAdmin is down, we may use this status page to include information about the issue and expected resolution."},
		{ID: MsgAboutSourceA, Other: "The source code of"},
		{ID: MsgAboutSourceLink, Other: "can be found on GitHub."},
	}

	ret := make([]*i18n.Message, len(messages))
	for i, message := range messages {
		copy := *message
		ret[i] = &copy
	}
	return ret
}
