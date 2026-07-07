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
	locations   *client.ActionLocationIndexResponse
	locationErr error
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

func (f *fakeOutageClient) ListLocations() (*client.ActionLocationIndexResponse, error) {
	if f.locationErr != nil {
		return nil, f.locationErr
	}
	if f.locations != nil {
		return f.locations, nil
	}
	return locationIndexResponse(true, nil), nil
}

func TestRefreshOutageReportsOnceMapsActiveRecentAndEntities(t *testing.T) {
	_, st, _ := newTestApplication(t)
	api := &fakeOutageClient{
		resp: outageIndexResponse(true, "", []*client.ActionOutageIndexOutput{
			apiOutage(1001, "planned_outage", "announced", fixedNow.Add(2*time.Hour), 90, "system_restart", "Router replacement"),
			apiOutage(1002, "unplanned_outage", "announced", fixedNow.Add(3*time.Hour), 45, "unavailability", "Switch down"),
			apiOutage(1003, "planned_outage", "resolved", fixedNow.Add(-2*time.Hour), 30, "system_restart", "Old maintenance"),
			apiOutage(1004, "unplanned_outage", "resolved", fixedNow.Add(-1*time.Hour), 15, "unavailability", "Recent outage"),
		}),
		entities: map[int64]*client.ActionOutageEntityIndexResponse{
			1001: outageEntityResponse(true, []*client.ActionOutageEntityIndexOutput{{Name: "node", EntityId: 101, Label: "node1.prg"}}),
			1004: outageEntityResponse(true, []*client.ActionOutageEntityIndexOutput{{Name: "location", EntityId: 3, Label: "Praha"}}),
		},
		locations: locationIndexResponse(true, []*client.ActionLocationIndexOutput{
			{Id: 7, Label: "Staging", Environment: &client.ActionEnvironmentShowOutput{Id: 5, Label: "Staging"}},
		}),
	}

	refreshOutageReportsOnce(st, api, fixedNow)

	if api.order != "oldest" || api.recentSince != fixedNow.AddDate(-1, 0, 0).Format(time.RFC3339) {
		t.Fatalf("ListOutages called with recentSince=%q order=%q", api.recentSince, api.order)
	}

	reports := st.OutageReports
	if !reports.Status || !reports.AnyActive || !reports.AnyActivePlanned || !reports.AnyActiveUnplanned || !reports.AnyRecent || !reports.AnyRecentPlanned || !reports.AnyRecentUnplanned {
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
	if got := reports.ActiveList[0].AffectedEntities[0].EntityType; got != "node" {
		t.Fatalf("active outage entity type = %q", got)
	}
	if got := reports.RecentList[0].AffectedEntities[0]; got.Name != "location" || got.Id != 3 || got.Label != "Praha" {
		t.Fatalf("recent outage entity = %+v", got)
	}
	if got := reports.RecentList[0].AffectedEntities[0].EntityType; got != "location" {
		t.Fatalf("recent outage entity type = %q", got)
	}
	if got := st.VpsAdminLocations[7]; got.Id != 7 || got.Label != "Staging" || got.EnvironmentId != 5 || got.EnvironmentLabel != "Staging" {
		t.Fatalf("vpsAdmin location metadata = %+v", got)
	}
}

func TestOutageEntityDisplayLabel(t *testing.T) {
	tests := []struct {
		name   string
		entity OutageEntity
		want   string
	}{
		{
			name:   "new API node label",
			entity: OutageEntity{Name: "Node", EntityType: "node", Label: "node1.prg"},
			want:   "node1.prg",
		},
		{
			name:   "old API node label",
			entity: OutageEntity{Name: "Node", Label: "Node node1.prg"},
			want:   "node1.prg",
		},
		{
			name:   "old API location label",
			entity: OutageEntity{Name: "Location", Label: "Location Praha"},
			want:   "Praha",
		},
		{
			name:   "custom label",
			entity: OutageEntity{Name: "External router", Label: "External router"},
			want:   "External router",
		},
		{
			name:   "empty label",
			entity: OutageEntity{Name: "External router"},
			want:   "External router",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.entity.DisplayLabel(); got != tt.want {
				t.Fatalf("DisplayLabel() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestOutageReportTitlesForLocale(t *testing.T) {
	catalog := newLocaleCatalog()
	cs, ok := catalog.localeForCode("cs", nil)
	if !ok {
		t.Fatal("Czech locale not found")
	}

	tests := []struct {
		name   string
		report OutageReports
		active string
		recent string
	}{
		{
			name: "planned",
			report: OutageReports{
				AnyActivePlanned: true,
				AnyRecentPlanned: true,
			},
			active: "Nahlášené odstávky",
			recent: "Nedávno ukončené odstávky",
		},
		{
			name: "unplanned",
			report: OutageReports{
				AnyActiveUnplanned: true,
				AnyRecentUnplanned: true,
			},
			active: "Nahlášené výpadky",
			recent: "Nedávno vyřešené výpadky",
		},
		{
			name: "mixed",
			report: OutageReports{
				AnyActivePlanned:   true,
				AnyActiveUnplanned: true,
				AnyRecentPlanned:   true,
				AnyRecentUnplanned: true,
			},
			active: "Nahlášené odstávky a výpadky",
			recent: "Nedávno ukončené odstávky a vyřešené výpadky",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.report.ActiveTitleForLocale(cs); got != tt.active {
				t.Fatalf("active title = %q, want %q", got, tt.active)
			}
			if got := tt.report.RecentTitleForLocale(cs); got != tt.recent {
				t.Fatalf("recent title = %q, want %q", got, tt.recent)
			}
		})
	}

	if got := cs.T("outages.unable_fetch"); got != "Nepodařilo se načíst hlášení odstávek a výpadků z vpsAdminu." {
		t.Fatalf("unable fetch text = %q", got)
	}

	if got := cs.TD("outage.summary.fallback", map[string]any{"ID": 123}); got != "Hlášení #123" {
		t.Fatalf("fallback summary = %q", got)
	}
}

func TestRefreshOutageReportsOnceKeepsOldReportsOnlyInHistory(t *testing.T) {
	_, st, _ := newTestApplication(t)
	api := &fakeOutageClient{
		resp: outageIndexResponse(true, "", []*client.ActionOutageIndexOutput{
			apiOutage(1001, "unplanned_outage", "resolved", fixedNow.Add(-10*24*time.Hour), 30, "unavailability", "Old outage"),
			apiOutage(1002, "unplanned_outage", "resolved", fixedNow.Add(-1*time.Hour), 15, "unavailability", "Recent outage"),
		}),
	}

	refreshOutageReportsOnce(st, api, fixedNow)

	if len(st.OutageReports.RecentList) != 1 || st.OutageReports.RecentList[0].Id != 1002 {
		t.Fatalf("recent reports = %+v", st.OutageReports.RecentList)
	}

	historyReports := st.History.OutageReports()
	if len(historyReports) != 2 {
		t.Fatalf("history reports = %+v, want 2", historyReports)
	}
}

func TestRefreshOutageReportsOnceFetchesAtLeastAvailabilityWindow(t *testing.T) {
	_, st, _ := newTestApplication(t)
	st.HistoryDays = 14
	api := &fakeOutageClient{
		resp: outageIndexResponse(true, "", nil),
	}

	refreshOutageReportsOnce(st, api, fixedNow)

	if api.recentSince != fixedNow.AddDate(-1, 0, 0).Format(time.RFC3339) {
		t.Fatalf("recentSince = %q, want 1 year availability window", api.recentSince)
	}
}

func TestRefreshOutageReportsOncePreservesLongerConfiguredHistoryWindow(t *testing.T) {
	_, st, _ := newTestApplication(t)
	st.HistoryDays = 400
	api := &fakeOutageClient{
		resp: outageIndexResponse(true, "", nil),
	}

	refreshOutageReportsOnce(st, api, fixedNow)

	if api.recentSince != fixedNow.AddDate(0, 0, -400).Format(time.RFC3339) {
		t.Fatalf("recentSince = %q, want 400 day configured history window", api.recentSince)
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

func TestRefreshOutageReportsOncePreservesStaleReportsOnAPIFailure(t *testing.T) {
	_, st, _ := newTestApplication(t)
	st.OutageReports = &OutageReports{
		Status:             true,
		AnyActive:          true,
		AnyActivePlanned:   true,
		AnyRecent:          true,
		AnyRecentUnplanned: true,
		ActiveList: []*OutageReport{
			{Id: 1001, Type: "planned_outage", State: "announced"},
		},
		RecentList: []*OutageReport{
			{Id: 1002, Type: "unplanned_outage", State: "resolved"},
		},
	}

	refreshOutageReportsOnce(st, &fakeOutageClient{err: errors.New("api failed")}, fixedNow)

	if st.OutageReports.Status {
		t.Fatal("outage status should be false after API error")
	}
	if !st.OutageReports.AnyActive || !st.OutageReports.AnyActivePlanned || !st.OutageReports.AnyRecent || !st.OutageReports.AnyRecentUnplanned {
		t.Fatalf("stale outage flags should be preserved after API failure: %+v", st.OutageReports)
	}
	if len(st.OutageReports.ActiveList) != 1 || st.OutageReports.ActiveList[0].Id != 1001 {
		t.Fatalf("stale active reports should be preserved after API failure: %+v", st.OutageReports.ActiveList)
	}
	if len(st.OutageReports.RecentList) != 1 || st.OutageReports.RecentList[0].Id != 1002 {
		t.Fatalf("stale recent reports should be preserved after API failure: %+v", st.OutageReports.RecentList)
	}
}

func TestRefreshOutageReportsOncePreservesOutageWhenEntityFetchFails(t *testing.T) {
	_, st, _ := newTestApplication(t)
	api := &fakeOutageClient{
		resp: outageIndexResponse(true, "", []*client.ActionOutageIndexOutput{
			apiOutage(1001, "unplanned_outage", "announced", fixedNow, 15, "unavailability", "Entity fetch failed"),
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

func TestRefreshOutageReportsOncePreservesOutagesWhenLocationFetchFails(t *testing.T) {
	_, st, _ := newTestApplication(t)
	api := &fakeOutageClient{
		resp: outageIndexResponse(true, "", []*client.ActionOutageIndexOutput{
			apiOutage(1001, "unplanned_outage", "announced", fixedNow, 15, "unavailability", "Location fetch failed"),
		}),
		locationErr: errors.New("locations failed"),
	}

	refreshOutageReportsOnce(st, api, fixedNow)

	if !st.OutageReports.Status || len(st.OutageReports.ActiveList) != 1 {
		t.Fatalf("outage should be preserved: %+v", st.OutageReports)
	}
	if got := st.OutageReports.ActiveList[0]; got.Id != 1001 {
		t.Fatalf("outage = %+v", got)
	}
}

func TestRefreshOutageReportsOnceHandlesMalformedBeginTime(t *testing.T) {
	_, st, _ := newTestApplication(t)
	outage := apiOutage(1001, "planned_outage", "announced", fixedNow, 15, "system_restart", "Bad time")
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

func locationIndexResponse(status bool, output []*client.ActionLocationIndexOutput) *client.ActionLocationIndexResponse {
	return &client.ActionLocationIndexResponse{
		Envelope: &client.Envelope{Status: status},
		Output:   output,
	}
}
