package main

import (
	"errors"
	"testing"
	"time"

	"github.com/vpsfreecz/vpsadmin-go-client/client"
)

type fakeSecurityAdvisoriesClient struct {
	resp        *client.ActionSecurityAdvisoryIndexResponse
	err         error
	cveResp     map[int64]*client.ActionSecurityAdvisoryCveIndexResponse
	cveErr      error
	recentSince string
	order       string
	limit       int64
	cveIds      []int64
}

func (f *fakeSecurityAdvisoriesClient) ListSecurityAdvisories(recentSince string, order string, limit int64) (*client.ActionSecurityAdvisoryIndexResponse, error) {
	f.recentSince = recentSince
	f.order = order
	f.limit = limit
	return f.resp, f.err
}

func (f *fakeSecurityAdvisoriesClient) ListSecurityAdvisoryCves(securityAdvisory int64) (*client.ActionSecurityAdvisoryCveIndexResponse, error) {
	f.cveIds = append(f.cveIds, securityAdvisory)
	if f.cveErr != nil {
		return nil, f.cveErr
	}
	if f.cveResp != nil && f.cveResp[securityAdvisory] != nil {
		return f.cveResp[securityAdvisory], nil
	}
	return securityAdvisoryCveIndexResponse(true, "", nil), nil
}

func TestRefreshSecurityAdvisoriesOnceMapsRecentAdvisories(t *testing.T) {
	_, st, _ := newTestApplication(t)
	api := &fakeSecurityAdvisoriesClient{
		resp: securityAdvisoryIndexResponse(true, "", []*client.ActionSecurityAdvisoryIndexOutput{
			apiSecurityAdvisory(
				2002,
				"published",
				"Dirty Pipe",
				fixedNow.Add(-2*time.Hour),
				"Kernel vulnerability was mitigated.",
				4,
			),
			apiSecurityAdvisory(
				2001,
				"retracted",
				"",
				fixedNow.Add(-3*time.Hour),
				"Advisory was retracted.",
				1,
			),
		}),
		cveResp: map[int64]*client.ActionSecurityAdvisoryCveIndexResponse{
			2002: securityAdvisoryCveIndexResponse(true, "", []*client.ActionSecurityAdvisoryCveIndexOutput{
				apiSecurityAdvisoryCve(3002, 2002, "CVE-2026-2002"),
			}),
			2001: securityAdvisoryCveIndexResponse(true, "", []*client.ActionSecurityAdvisoryCveIndexOutput{
				apiSecurityAdvisoryCve(3001, 2001, "CVE-2026-2001"),
			}),
		},
	}

	refreshSecurityAdvisoriesOnce(st, api, fixedNow)

	if api.order != "newest" || api.limit != securityAdvisoryRecentLimit || api.recentSince != fixedNow.AddDate(0, 0, -securityAdvisoryRecentDays).Format(time.RFC3339) {
		t.Fatalf("ListSecurityAdvisories called with recentSince=%q order=%q limit=%d", api.recentSince, api.order, api.limit)
	}

	advisories := st.SecurityAdvisories
	if !advisories.Status || !advisories.AnyRecent {
		t.Fatalf("security advisory flags = %+v", advisories)
	}
	if len(advisories.RecentList) != 2 || advisories.RecentList[0].Id != 2002 || advisories.RecentList[1].Id != 2001 {
		t.Fatalf("recent security advisories = %+v", advisories.RecentList)
	}
	if len(api.cveIds) != 2 || api.cveIds[0] != 2002 || api.cveIds[1] != 2001 {
		t.Fatalf("ListSecurityAdvisoryCves called with ids=%v", api.cveIds)
	}
	if got := advisories.RecentList[0]; len(got.Cves) != 1 || got.Cves[0].CveId != "CVE-2026-2002" || got.Name != "Dirty Pipe" || got.AffectedNodeCount != 4 {
		t.Fatalf("security advisory = %+v", got)
	}
}

func TestRefreshSecurityAdvisoriesOnceHandlesAPIFailure(t *testing.T) {
	tests := []struct {
		name string
		api  *fakeSecurityAdvisoriesClient
	}{
		{
			name: "request error",
			api:  &fakeSecurityAdvisoriesClient{err: errors.New("api failed")},
		},
		{
			name: "failed response",
			api: &fakeSecurityAdvisoriesClient{
				resp: securityAdvisoryIndexResponse(false, "not available", nil),
			},
		},
		{
			name: "cve request error",
			api: &fakeSecurityAdvisoriesClient{
				resp: securityAdvisoryIndexResponse(true, "", []*client.ActionSecurityAdvisoryIndexOutput{
					apiSecurityAdvisory(
						2001,
						"published",
						"",
						fixedNow,
						"Missing CVEs",
						1,
					),
				}),
				cveErr: errors.New("cve api failed"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, st, _ := newTestApplication(t)
			st.SecurityAdvisories.Status = true

			refreshSecurityAdvisoriesOnce(st, tt.api, fixedNow)

			if st.SecurityAdvisories.Status {
				t.Fatal("security advisory status should be false after API error")
			}
		})
	}
}

func TestRefreshSecurityAdvisoriesOncePreservesStaleAdvisoriesOnAPIFailure(t *testing.T) {
	_, st, _ := newTestApplication(t)
	st.SecurityAdvisories = &SecurityAdvisories{
		Status:    true,
		AnyRecent: true,
		RecentList: []*SecurityAdvisory{
			{
				Id:    2001,
				Cves:  []SecurityAdvisoryCve{{Id: 3001, CveId: "CVE-2026-2001"}},
				State: "published",
			},
		},
	}

	refreshSecurityAdvisoriesOnce(st, &fakeSecurityAdvisoriesClient{err: errors.New("api failed")}, fixedNow)

	if st.SecurityAdvisories.Status {
		t.Fatal("security advisory status should be false after API error")
	}
	if !st.SecurityAdvisories.AnyRecent {
		t.Fatalf("stale security advisory flags should be preserved after API failure: %+v", st.SecurityAdvisories)
	}
	if len(st.SecurityAdvisories.RecentList) != 1 || st.SecurityAdvisories.RecentList[0].Id != 2001 {
		t.Fatalf("stale security advisories should be preserved after API failure: %+v", st.SecurityAdvisories.RecentList)
	}
}

func TestRefreshSecurityAdvisoriesOnceHandlesMalformedTime(t *testing.T) {
	_, st, _ := newTestApplication(t)
	advisory := apiSecurityAdvisory(
		2001,
		"published",
		"",
		fixedNow,
		"Bad time",
		1,
	)
	advisory.PublishedAt = "not-a-time"
	api := &fakeSecurityAdvisoriesClient{
		resp: securityAdvisoryIndexResponse(true, "", []*client.ActionSecurityAdvisoryIndexOutput{advisory}),
		cveResp: map[int64]*client.ActionSecurityAdvisoryCveIndexResponse{
			2001: securityAdvisoryCveIndexResponse(true, "", []*client.ActionSecurityAdvisoryCveIndexOutput{
				apiSecurityAdvisoryCve(3001, 2001, "CVE-2026-2001"),
			}),
		},
	}

	refreshSecurityAdvisoriesOnce(st, api, fixedNow)

	if !st.SecurityAdvisories.Status || len(st.SecurityAdvisories.RecentList) != 1 {
		t.Fatalf("security advisory should be preserved: %+v", st.SecurityAdvisories)
	}
	if got := st.SecurityAdvisories.RecentList[0].PublishedAt; !got.IsZero() {
		t.Fatalf("PublishedAt = %s, want zero time", got)
	}
}

func securityAdvisoryIndexResponse(status bool, message string, output []*client.ActionSecurityAdvisoryIndexOutput) *client.ActionSecurityAdvisoryIndexResponse {
	return &client.ActionSecurityAdvisoryIndexResponse{
		Envelope: &client.Envelope{Status: status, Message: message},
		Output:   output,
	}
}

func securityAdvisoryCveIndexResponse(status bool, message string, output []*client.ActionSecurityAdvisoryCveIndexOutput) *client.ActionSecurityAdvisoryCveIndexResponse {
	return &client.ActionSecurityAdvisoryCveIndexResponse{
		Envelope: &client.Envelope{Status: status, Message: message},
		Output:   output,
	}
}

func apiSecurityAdvisory(id int64, state string, name string, publishedAt time.Time, summary string, affectedNodes int64) *client.ActionSecurityAdvisoryIndexOutput {
	return &client.ActionSecurityAdvisoryIndexOutput{
		Id:                id,
		State:             state,
		Name:              name,
		PublishedAt:       apiTimestamp(publishedAt),
		UpdatedAt:         apiTimestamp(publishedAt.Add(30 * time.Minute)),
		EnSummary:         summary,
		EnDescription:     summary + " description",
		EnResponse:        summary + " response",
		AffectedNodeCount: affectedNodes,
	}
}

func apiSecurityAdvisoryCve(id int64, advisoryId int64, cveId string) *client.ActionSecurityAdvisoryCveIndexOutput {
	return &client.ActionSecurityAdvisoryCveIndexOutput{
		Id:                 id,
		SecurityAdvisoryId: advisoryId,
		CveId:              cveId,
		Url:                "https://www.cve.org/CVERecord?id=" + cveId,
	}
}
