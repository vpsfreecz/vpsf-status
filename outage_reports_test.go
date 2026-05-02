package main

import (
	"errors"
	"testing"
	"time"

	"github.com/vpsfreecz/vpsadmin-go-client/client"
)

type fakeOutageClient struct {
	resp        *client.ActionOutageIndexResponse
	err         error
	entities    map[int64]*client.ActionOutageEntityIndexResponse
	recentSince string
	order       string
}

func (f *fakeOutageClient) ListOutages(recentSince string, order string) (*client.ActionOutageIndexResponse, error) {
	f.recentSince = recentSince
	f.order = order
	return f.resp, f.err
}

func (f *fakeOutageClient) ListOutageEntities(outageID int64) (*client.ActionOutageEntityIndexResponse, error) {
	if f.entities == nil {
		return outageEntityResponse(true, nil), nil
	}

	if resp := f.entities[outageID]; resp != nil {
		return resp, nil
	}

	return outageEntityResponse(true, nil), nil
}

func TestRefreshOutageReportsOnceMapsActiveRecentAndEntities(t *testing.T) {
	_, st, _ := newTestApplication(t)
	api := &fakeOutageClient{
		resp: outageIndexResponse(true, "", []*client.ActionOutageIndexOutput{
			apiOutage(1001, "maintenance", "announced", fixedNow.Add(2*time.Hour), 90, "partial", "Router replacement"),
			apiOutage(1002, "outage", "announced", fixedNow.Add(3*time.Hour), 45, "full", "Switch down"),
			apiOutage(1003, "maintenance", "resolved", fixedNow.Add(-2*time.Hour), 30, "partial", "Old maintenance"),
			apiOutage(1004, "outage", "resolved", fixedNow.Add(-1*time.Hour), 15, "full", "Recent outage"),
		}),
		entities: map[int64]*client.ActionOutageEntityIndexResponse{
			1001: outageEntityResponse(true, []*client.ActionOutageEntityIndexOutput{{Name: "node", EntityId: 101, Label: "node1.prg"}}),
			1004: outageEntityResponse(true, []*client.ActionOutageEntityIndexOutput{{Name: "location", EntityId: 3, Label: "Praha"}}),
		},
	}

	refreshOutageReportsOnce(st, api, fixedNow)

	if api.order != "oldest" || api.recentSince != fixedNow.AddDate(0, 0, -2).Format(time.RFC3339) {
		t.Fatalf("ListOutages called with recentSince=%q order=%q", api.recentSince, api.order)
	}

	reports := st.OutageReports
	if !reports.Status || !reports.AnyActive || !reports.AnyActiveMaintenance || !reports.AnyActiveOutage || !reports.AnyRecent || !reports.AnyRecentMaintenance || !reports.AnyRecentOutage {
		t.Fatalf("outage flags = %+v", reports)
	}
	if len(reports.ActiveList) != 2 || reports.ActiveList[0].Id != 1001 || reports.ActiveList[1].Id != 1002 {
		t.Fatalf("active outages = %+v", reports.ActiveList)
	}
	if len(reports.RecentList) != 2 || reports.RecentList[0].Id != 1004 || reports.RecentList[1].Id != 1003 {
		t.Fatalf("recent outages = %+v", reports.RecentList)
	}
	if got := reports.ActiveList[0].AffectedEntities[0]; got.Name != "node" || got.Id != 101 || got.Label != "node1.prg" {
		t.Fatalf("active outage entity = %+v", got)
	}
	if got := reports.RecentList[0].AffectedEntities[0]; got.Name != "location" || got.Id != 3 || got.Label != "Praha" {
		t.Fatalf("recent outage entity = %+v", got)
	}
}

func TestRefreshOutageReportsOnceHandlesAPIFailure(t *testing.T) {
	tests := []struct {
		name string
		api  *fakeOutageClient
	}{
		{
			name: "request error",
			api:  &fakeOutageClient{err: errors.New("api failed")},
		},
		{
			name: "failed response",
			api: &fakeOutageClient{
				resp: outageIndexResponse(false, "not available", nil),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, st, _ := newTestApplication(t)
			st.OutageReports.Status = true

			refreshOutageReportsOnce(st, tt.api, fixedNow)

			if st.OutageReports.Status {
				t.Fatal("outage status should be false after API error")
			}
		})
	}
}

func TestRefreshOutageReportsOncePreservesOutageWhenEntityFetchFails(t *testing.T) {
	_, st, _ := newTestApplication(t)
	api := &fakeOutageClient{
		resp: outageIndexResponse(true, "", []*client.ActionOutageIndexOutput{
			apiOutage(1001, "outage", "announced", fixedNow, 15, "full", "Entity fetch failed"),
		}),
		entities: map[int64]*client.ActionOutageEntityIndexResponse{
			1001: outageEntityResponse(false, nil),
		},
	}

	refreshOutageReportsOnce(st, api, fixedNow)

	if !st.OutageReports.Status || len(st.OutageReports.ActiveList) != 1 {
		t.Fatalf("outage should be preserved: %+v", st.OutageReports)
	}
	if got := st.OutageReports.ActiveList[0]; got.Id != 1001 || len(got.AffectedEntities) != 0 {
		t.Fatalf("outage = %+v", got)
	}
}

func TestRefreshOutageReportsOnceHandlesMalformedBeginTime(t *testing.T) {
	_, st, _ := newTestApplication(t)
	outage := apiOutage(1001, "maintenance", "announced", fixedNow, 15, "partial", "Bad time")
	outage.BeginsAt = "not-a-time"
	api := &fakeOutageClient{
		resp: outageIndexResponse(true, "", []*client.ActionOutageIndexOutput{outage}),
	}

	refreshOutageReportsOnce(st, api, fixedNow)

	if !st.OutageReports.Status || len(st.OutageReports.ActiveList) != 1 {
		t.Fatalf("outage should be preserved: %+v", st.OutageReports)
	}
	if got := st.OutageReports.ActiveList[0].BeginsAt; !got.IsZero() {
		t.Fatalf("BeginsAt = %s, want zero time", got)
	}
}

func outageIndexResponse(status bool, message string, output []*client.ActionOutageIndexOutput) *client.ActionOutageIndexResponse {
	return &client.ActionOutageIndexResponse{
		Envelope: &client.Envelope{Status: status, Message: message},
		Output:   output,
	}
}

func apiOutage(id int64, outageType string, state string, beginsAt time.Time, duration int64, impact string, summary string) *client.ActionOutageIndexOutput {
	return &client.ActionOutageIndexOutput{
		Id:            id,
		Type:          outageType,
		State:         state,
		BeginsAt:      apiTimestamp(beginsAt),
		Duration:      duration,
		Impact:        impact,
		CsSummary:     summary + " CS",
		CsDescription: summary + " CS description",
		EnSummary:     summary,
		EnDescription: summary + " description",
	}
}

func outageEntityResponse(status bool, output []*client.ActionOutageEntityIndexOutput) *client.ActionOutageEntityIndexResponse {
	return &client.ActionOutageEntityIndexResponse{
		Envelope: &client.Envelope{Status: status},
		Output:   output,
	}
}
