package main

import (
	"fmt"
	"log"
	"slices"
	"time"

	"github.com/vpsfreecz/vpsadmin-go-client/client"
)

func checkOutageReports(st *Status, checkInterval time.Duration) {
	api := client.New(st.VpsAdmin.Api.Url)

	for {
		list := api.Outage.Index.Prepare()

		input := list.NewInput()
		input.SetRecentSince(time.Now().AddDate(0, 0, -2).Format(time.RFC3339))
		input.SetOrder("oldest")

		resp, err := list.Call()

		if err != nil {
			log.Printf("Unable to fetch outages: %+v", err)
			failOutages(st)
			time.Sleep(checkInterval)
			continue
		} else if !resp.Status {
			log.Printf("Failed to list outages: %s", resp.Message)
			failOutages(st)
			time.Sleep(checkInterval)
			continue
		}

		reports := &OutageReports{
			Status:     true,
			ActiveList: make([]*OutageReport, 0),
			RecentList: make([]*OutageReport, 0),
		}

		for _, outage := range resp.Output {
			v := OutageReport{
				Id:               outage.Id,
				Duration:         time.Duration(outage.Duration) * time.Minute,
				Type:             outage.Type,
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

			if err := fetchOutageEntities(api, &v); err != nil {
				log.Printf("Unable to fetch entities of outage #%d: %+v", v.Id, err)
			}

			if v.State == "announced" {
				reports.AnyActive = true

				if v.Type == "maintenance" {
					reports.AnyActiveMaintenance = true
				} else {
					reports.AnyActiveOutage = true
				}

				reports.ActiveList = append(reports.ActiveList, &v)
			} else {
				reports.AnyRecent = true

				if v.Type == "maintenance" {
					reports.AnyRecentMaintenance = true
				} else {
					reports.AnyRecentOutage = true
				}

				reports.RecentList = append(reports.RecentList, &v)
			}

			slices.Reverse(reports.RecentList)
		}

		st.OutageReports = reports
		time.Sleep(checkInterval)
	}
}

func failOutages(st *Status) {
	st.OutageReports.Status = false
}

func fetchOutageEntities(api *client.Client, report *OutageReport) error {
	list := api.Outage.Entity.Index.Prepare()
	list.SetPathParamInt("outage_id", report.Id)

	resp, err := list.Call()

	if err != nil {
		return err
	} else if !resp.Status {
		return fmt.Errorf("failed to list outage entities: %s", resp.Message)
	}

	for _, entity := range resp.Output {
		report.AffectedEntities = append(report.AffectedEntities, OutageEntity{
			Name:  entity.Name,
			Id:    entity.Id,
			Label: entity.Label,
		})
	}

	return nil
}
