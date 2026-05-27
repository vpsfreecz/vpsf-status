package main

import (
	"fmt"
	"log"
	"slices"
	"time"

	"github.com/vpsfreecz/vpsadmin-go-client/client"
)

type outageReportsClient interface {
	ListOutages(recentSince string, order string) (*client.ActionOutageIndexResponse, error)
	ListOutageEntities(outageID int64) (*client.ActionOutageEntityIndexResponse, error)
	ListLocations() (*client.ActionLocationIndexResponse, error)
}

type liveOutageReportsClient struct {
	api *client.Client
}

func (c liveOutageReportsClient) ListOutages(recentSince string, order string) (*client.ActionOutageIndexResponse, error) {
	list := c.api.Outage.Index.Prepare()

	input := list.NewInput()
	input.SetRecentSince(recentSince)
	input.SetOrder(order)

	return list.Call()
}

func (c liveOutageReportsClient) ListOutageEntities(outageID int64) (*client.ActionOutageEntityIndexResponse, error) {
	list := c.api.Outage.Entity.Index.Prepare()
	list.SetPathParamInt("outage_id", outageID)

	return list.Call()
}

func (c liveOutageReportsClient) ListLocations() (*client.ActionLocationIndexResponse, error) {
	return c.api.Location.Index.Prepare().Call()
}

func checkOutageReports(st *Status, checkInterval time.Duration, checkTimeout time.Duration) {
	api := liveOutageReportsClient{api: newVpsAdminClient(st.VpsAdmin.Api.Url, checkTimeout)}

	for {
		now := time.Now()
		refreshOutageReportsOnce(st, api, now)
		sleepUntilNextProbe(now, checkInterval)
	}
}

func refreshOutageReportsOnce(st *Status, api outageReportsClient, now time.Time) {
	resp, err := api.ListOutages(outageReportFetchStart(st, now).Format(time.RFC3339), "oldest")

	if err != nil {
		log.Printf("Unable to fetch outages: %+v", err)
		failOutages(st)
		return
	} else if !resp.Status {
		log.Printf("Failed to list outages: %s", resp.Message)
		failOutages(st)
		return
	}

	if locations, err := fetchVpsAdminLocations(api); err != nil {
		log.Printf("Unable to fetch vpsAdmin locations: %+v", err)
	} else {
		st.VpsAdminLocations = locations
	}

	allReports := make([]*OutageReport, 0, len(resp.Output))

	for _, outage := range resp.Output {
		v := OutageReport{
			Id:               outage.Id,
			Duration:         time.Duration(outage.Duration) * time.Minute,
			Type:             normalizeOutageType(outage.Type),
			State:            outage.State,
			Impact:           outage.Impact,
			CsSummary:        outage.CsSummary,
			CsDescription:    outage.CsDescription,
			EnSummary:        outage.EnSummary,
			EnDescription:    outage.EnDescription,
			AffectedEntities: make([]OutageEntity, 0),
		}

		beginsAt, err := time.Parse("2006-01-02T15:04:05Z", outage.BeginsAt)
		if err == nil {
			v.BeginsAt = beginsAt
		} else {
			log.Printf("Unable to parse outage time %v: %+v", outage.BeginsAt, err)
		}

		if outage.FinishedAt != "" {
			finishedAt, err := time.Parse("2006-01-02T15:04:05Z", outage.FinishedAt)
			if err == nil {
				v.FinishedAt = finishedAt
			} else {
				log.Printf("Unable to parse outage finish time %v: %+v", outage.FinishedAt, err)
			}
		}

		if err := fetchOutageEntities(api, &v); err != nil {
			log.Printf("Unable to fetch entities of outage #%d: %+v", v.Id, err)
		}

		allReports = append(allReports, &v)
	}

	if st.History != nil {
		if err := st.History.ReplaceOutages(allReports, now); err != nil {
			log.Printf("Unable to store outage history: %+v", err)
		}
	}

	reports := createCurrentOutageReports(allReports, now)
	slices.Reverse(reports.RecentList)
	st.OutageReports = reports
}

func outageReportFetchStart(st *Status, now time.Time) time.Time {
	return availabilityFetchStart(now, historyDaysForStatus(st))
}

func createCurrentOutageReports(allReports []*OutageReport, now time.Time) *OutageReports {
	reports := &OutageReports{
		Status:     true,
		ActiveList: make([]*OutageReport, 0),
		RecentList: make([]*OutageReport, 0),
	}

	recentSince := now.AddDate(0, 0, -2)

	for _, report := range allReports {
		if report.State == "announced" {
			reports.AnyActive = true

			if report.IsPlannedOutage() {
				reports.AnyActivePlanned = true
			} else {
				reports.AnyActiveUnplanned = true
			}

			reports.ActiveList = append(reports.ActiveList, report)
		} else if !report.BeginsAt.IsZero() && !report.BeginsAt.Before(recentSince) {
			reports.AnyRecent = true

			if report.IsPlannedOutage() {
				reports.AnyRecentPlanned = true
			} else {
				reports.AnyRecentUnplanned = true
			}

			reports.RecentList = append(reports.RecentList, report)
		}
	}

	return reports
}

func failOutages(st *Status) {
	st.OutageReports.Status = false
}

func fetchOutageEntities(api outageReportsClient, report *OutageReport) error {
	resp, err := api.ListOutageEntities(report.Id)
	if err != nil {
		return err
	} else if !resp.Status {
		return fmt.Errorf("failed to list outage entities: %s", resp.Message)
	}

	for _, entity := range resp.Output {
		report.AffectedEntities = append(report.AffectedEntities, OutageEntity{
			Name:  entity.Name,
			Id:    entity.EntityId,
			Label: entity.Label,
		})
	}

	return nil
}

func fetchVpsAdminLocations(api outageReportsClient) (map[int64]VpsAdminLocation, error) {
	resp, err := api.ListLocations()
	if err != nil {
		return nil, err
	} else if !resp.Status {
		return nil, fmt.Errorf("failed to list locations: %s", resp.Message)
	}

	ret := make(map[int64]VpsAdminLocation, len(resp.Output))
	for _, loc := range resp.Output {
		if loc == nil {
			continue
		}

		v := VpsAdminLocation{
			Id:    loc.Id,
			Label: loc.Label,
		}

		if loc.Environment != nil {
			v.EnvironmentId = loc.Environment.Id
			v.EnvironmentLabel = loc.Environment.Label
		}

		ret[v.Id] = v
	}

	return ret, nil
}
