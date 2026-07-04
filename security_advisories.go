package main

import (
	"log"
	"strings"
	"time"

	"github.com/vpsfreecz/vpsadmin-go-client/client"
)

const (
	securityAdvisoryRecentDays  = 30
	securityAdvisoryRecentLimit = 10
)

type securityAdvisoriesClient interface {
	ListSecurityAdvisories(recentSince string, order string, limit int64) (*client.ActionSecurityAdvisoryIndexResponse, error)
	ListSecurityAdvisoryCves(securityAdvisory int64) (*client.ActionSecurityAdvisoryCveIndexResponse, error)
}

type liveSecurityAdvisoriesClient struct {
	api *client.Client
}

func (c liveSecurityAdvisoriesClient) ListSecurityAdvisories(recentSince string, order string, limit int64) (*client.ActionSecurityAdvisoryIndexResponse, error) {
	list := c.api.SecurityAdvisory.Index.Prepare()

	input := list.NewInput()
	input.SetRecentSince(recentSince)
	input.SetOrder(order)
	input.SetLimit(limit)

	return list.Call()
}

func (c liveSecurityAdvisoriesClient) ListSecurityAdvisoryCves(securityAdvisory int64) (*client.ActionSecurityAdvisoryCveIndexResponse, error) {
	list := c.api.SecurityAdvisoryCve.Index.Prepare()

	input := list.NewInput()
	input.SetSecurityAdvisory(securityAdvisory)

	return list.Call()
}

func checkSecurityAdvisories(st *Status, checkInterval time.Duration, checkTimeout time.Duration) {
	api := liveSecurityAdvisoriesClient{api: newVpsAdminClient(st.VpsAdmin.Api.Url, checkTimeout)}

	for {
		now := time.Now()
		refreshSecurityAdvisoriesOnce(st, api, now)
		sleepUntilNextProbe(now, checkInterval)
	}
}

func refreshSecurityAdvisoriesOnce(st *Status, api securityAdvisoriesClient, now time.Time) {
	resp, err := api.ListSecurityAdvisories(
		securityAdvisoryFetchStart(now).Format(time.RFC3339),
		"newest",
		securityAdvisoryRecentLimit,
	)

	if err != nil {
		log.Printf("Unable to fetch security advisories: %+v", err)
		failSecurityAdvisories(st)
		return
	} else if !resp.Status {
		log.Printf("Failed to list security advisories: %s", resp.Message)
		failSecurityAdvisories(st)
		return
	}

	advisories := make([]*SecurityAdvisory, 0, len(resp.Output))
	for _, advisory := range resp.Output {
		v := SecurityAdvisory{
			Id:                advisory.Id,
			State:             advisory.State,
			Name:              advisory.Name,
			CsSummary:         advisory.CsSummary,
			CsDescription:     advisory.CsDescription,
			CsResponse:        advisory.CsResponse,
			EnSummary:         advisory.EnSummary,
			EnDescription:     advisory.EnDescription,
			EnResponse:        advisory.EnResponse,
			AffectedNodeCount: advisory.AffectedNodeCount,
		}

		if err := parseSecurityAdvisoryTime(advisory.PublishedAt, &v.PublishedAt); err != nil {
			log.Printf("Unable to parse security advisory published_at %v: %+v", advisory.PublishedAt, err)
		}

		if err := parseSecurityAdvisoryTime(advisory.UpdatedAt, &v.UpdatedAt); err != nil {
			log.Printf("Unable to parse security advisory updated_at %v: %+v", advisory.UpdatedAt, err)
		}

		cves, err := fetchSecurityAdvisoryCves(api, advisory.Id)
		if err != nil {
			log.Printf("Unable to fetch CVEs for security advisory %d: %+v", advisory.Id, err)
			failSecurityAdvisories(st)
			return
		}
		v.Cves = cves

		advisories = append(advisories, &v)
	}

	st.SecurityAdvisories = createCurrentSecurityAdvisories(advisories)
	st.requestIndexRenderIfConfigured()
}

func securityAdvisoryFetchStart(now time.Time) time.Time {
	return now.AddDate(0, 0, -securityAdvisoryRecentDays)
}

func createCurrentSecurityAdvisories(advisories []*SecurityAdvisory) *SecurityAdvisories {
	return &SecurityAdvisories{
		Status:     true,
		AnyRecent:  len(advisories) > 0,
		RecentList: advisories,
	}
}

func failSecurityAdvisories(st *Status) {
	st.SecurityAdvisories.Status = false
	st.requestIndexRenderIfConfigured()
}

func fetchSecurityAdvisoryCves(api securityAdvisoriesClient, advisoryId int64) ([]SecurityAdvisoryCve, error) {
	resp, err := api.ListSecurityAdvisoryCves(advisoryId)
	if err != nil {
		return nil, err
	} else if !resp.Status {
		return nil, &securityAdvisoryFetchError{message: resp.Message}
	}

	ret := make([]SecurityAdvisoryCve, len(resp.Output))
	for i, cve := range resp.Output {
		ret[i] = SecurityAdvisoryCve{
			Id:    cve.Id,
			CveId: cve.CveId,
			Url:   cve.Url,
		}
	}

	return ret, nil
}

type securityAdvisoryFetchError struct {
	message string
}

func (e *securityAdvisoryFetchError) Error() string {
	return e.message
}

func parseSecurityAdvisoryTime(value string, target *time.Time) error {
	if value == "" {
		return nil
	}

	t, err := time.Parse("2006-01-02T15:04:05Z", value)
	if err != nil {
		return err
	}

	*target = t
	return nil
}

func (a *SecurityAdvisory) CveLabel() string {
	if a == nil {
		return ""
	}

	cves := make([]string, 0, len(a.Cves))
	for _, cve := range a.Cves {
		cves = append(cves, cve.CveId)
	}

	label := strings.Join(cves, ", ")
	if label != "" && a.Name != "" {
		return label + " (" + a.Name + ")"
	}
	if label != "" {
		return label
	}
	return a.Name
}

func (a *SecurityAdvisory) SummaryForLocale(loc *pageLocale) string {
	if a == nil {
		return ""
	}

	summary := a.EnSummary
	if loc != nil && loc.Code == "cs" {
		summary = a.CsSummary
	}
	if summary == "" {
		summary = a.EnSummary
	}
	if summary == "" {
		summary = a.CsSummary
	}
	return summary
}
