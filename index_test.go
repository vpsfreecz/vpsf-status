package main

import (
	"html/template"
	"net/http"
	"testing"
	"time"
)

func TestIndexRefreshSkipsUnchangedBody(t *testing.T) {
	now := fixedNow
	app, st, _ := newTestApplication(t)
	app.now = func() time.Time {
		return now
	}
	setOperationalFixture(st)

	first, rendered, err := app.refreshIndexBody(now, true)
	if err != nil {
		t.Fatalf("render index body: %v", err)
	}
	if !rendered {
		t.Fatal("forced index body render was skipped")
	}

	now = fixedNow.Add(2 * time.Second)
	second, rendered, err := app.refreshIndexBody(now, false)
	if err != nil {
		t.Fatalf("refresh index body: %v", err)
	}
	if rendered {
		t.Fatal("unchanged index body was rendered")
	}
	if string(second.body) != string(first.body) {
		t.Fatal("skipped render returned different body")
	}
	if !second.renderedAt.Equal(fixedNow) {
		t.Fatalf("skipped render renderedAt = %s, want %s", second.renderedAt, fixedNow)
	}

	families := scrapeMetrics(t, app)
	requireUnlabeledMetricValue(t, families["vpsfstatus_index_last_render_timestamp_seconds"], float64(fixedNow.Unix()))
	requireUnlabeledMetricValue(t, families["vpsfstatus_index_last_render_attempt_timestamp_seconds"], float64(now.Unix()))
	requireUnlabeledCounterValue(t, families["vpsfstatus_index_render_skips_total"], 1)
}

func TestIndexRefreshRendersChangedBody(t *testing.T) {
	now := fixedNow
	app, st, _ := newTestApplication(t)
	app.now = func() time.Time {
		return now
	}
	setOperationalFixture(st)

	if _, _, err := app.refreshIndexBody(now, true); err != nil {
		t.Fatalf("render index body: %v", err)
	}

	setWebServiceState(st, st.Services.Web[1], false, false, http.StatusInternalServerError)
	now = fixedNow.Add(2 * time.Second)

	body, rendered, err := app.refreshIndexBody(now, false)
	if err != nil {
		t.Fatalf("refresh index body: %v", err)
	}
	if !rendered {
		t.Fatal("changed index body was skipped")
	}
	if !body.renderedAt.Equal(now) {
		t.Fatalf("changed render renderedAt = %s, want %s", body.renderedAt, now)
	}
	requireContains(t, string(body.body), `aria-label="Down"`)
}

func TestIndexRefreshRendersChangedCzechSummaries(t *testing.T) {
	now := fixedNow
	app, st, _ := newTestApplication(t)
	app.now = func() time.Time {
		return now
	}
	setOperationalFixture(st)
	addOutageFixture(st)
	addSecurityAdvisoryFixture(st)

	st.OutageReports.ActiveList[0].CsSummary = "Původní výměna routeru"
	st.SecurityAdvisories.RecentList[0].CsSummary = "Původní bezpečnostní oznámení."

	app.ensureLocales()
	loc, ok := app.locales.localeForCode("cs", nil)
	if !ok {
		t.Fatal("Czech locale is not available")
	}

	body, rendered, err := app.refreshIndexBody(now, true, loc)
	if err != nil {
		t.Fatalf("render Czech index body: %v", err)
	}
	if !rendered {
		t.Fatal("forced Czech index body render was skipped")
	}
	requireContains(t, string(body.body), "Původní výměna routeru", "Původní bezpečnostní oznámení.")

	st.OutageReports.ActiveList[0].CsSummary = "Nová výměna routeru"
	now = fixedNow.Add(2 * time.Second)
	body, rendered, err = app.refreshIndexBody(now, false, loc)
	if err != nil {
		t.Fatalf("refresh Czech index body after outage text change: %v", err)
	}
	if !rendered {
		t.Fatal("Czech index body was skipped after outage summary changed")
	}
	requireContains(t, string(body.body), "Nová výměna routeru")
	requireNotContains(t, string(body.body), "Původní výměna routeru")

	st.SecurityAdvisories.RecentList[0].CsSummary = "Nové bezpečnostní oznámení."
	now = fixedNow.Add(4 * time.Second)
	body, rendered, err = app.refreshIndexBody(now, false, loc)
	if err != nil {
		t.Fatalf("refresh Czech index body after advisory text change: %v", err)
	}
	if !rendered {
		t.Fatal("Czech index body was skipped after security advisory summary changed")
	}
	requireContains(t, string(body.body), "Nové bezpečnostní oznámení.")
	requireNotContains(t, string(body.body), "Původní bezpečnostní oznámení.")
}

func TestIndexRefreshRendersChangedLocalizedDescription(t *testing.T) {
	now := fixedNow
	app, st, _ := newTestApplication(t)
	app.now = func() time.Time {
		return now
	}
	setOperationalFixture(st)

	app.ensureLocales()
	loc, ok := app.locales.localeForCode("cs", nil)
	if !ok {
		t.Fatal("Czech locale is not available")
	}

	body, rendered, err := app.refreshIndexBody(now, true, loc)
	if err != nil {
		t.Fatalf("render Czech index body: %v", err)
	}
	if !rendered {
		t.Fatal("forced Czech index body render was skipped")
	}
	requireContains(t, string(body.body), "Webové stránky v češtině")

	st.Services.Web[0].Descriptions["cs"] = "České webové stránky"
	now = fixedNow.Add(2 * time.Second)
	body, rendered, err = app.refreshIndexBody(now, false, loc)
	if err != nil {
		t.Fatalf("refresh Czech index body after description change: %v", err)
	}
	if !rendered {
		t.Fatal("Czech index body was skipped after localized description changed")
	}
	requireContains(t, string(body.body), "České webové stránky")
	requireNotContains(t, string(body.body), "Webové stránky v češtině")
}

func TestIndexRefreshKeepaliveRendersUnchangedBody(t *testing.T) {
	now := fixedNow
	app, st, _ := newTestApplication(t)
	app.now = func() time.Time {
		return now
	}
	setOperationalFixture(st)

	if _, _, err := app.refreshIndexBody(now, true); err != nil {
		t.Fatalf("render index body: %v", err)
	}

	now = fixedNow.Add(indexRenderKeepalive - time.Second)
	if _, rendered, err := app.refreshIndexBody(now, false); err != nil {
		t.Fatalf("refresh index body before keepalive: %v", err)
	} else if rendered {
		t.Fatal("index body rendered before keepalive interval")
	}

	now = fixedNow.Add(indexRenderKeepalive)
	body, rendered, err := app.refreshIndexBody(now, false)
	if err != nil {
		t.Fatalf("refresh index body at keepalive: %v", err)
	}
	if !rendered {
		t.Fatal("index body was not rendered at keepalive interval")
	}
	if !body.renderedAt.Equal(now) {
		t.Fatalf("keepalive renderedAt = %s, want %s", body.renderedAt, now)
	}
}

func TestIndexRenderThrottleDelay(t *testing.T) {
	index := newPreRenderedIndex()
	index.setLastAttempt(fixedNow)

	if got, want := index.nextRenderDelay(fixedNow.Add(400*time.Millisecond)), 600*time.Millisecond; got != want {
		t.Fatalf("next render delay = %s, want %s", got, want)
	}
	if got := index.nextRenderDelay(fixedNow.Add(indexRenderMinInterval)); got != 0 {
		t.Fatalf("next render delay after interval = %s, want 0", got)
	}
}

func TestIndexRenderFailureKeepsLastGoodBody(t *testing.T) {
	now := fixedNow
	app, st, _ := newTestApplication(t)
	app.now = func() time.Time {
		return now
	}
	setOperationalFixture(st)

	first, _, err := app.refreshIndexBody(now, true)
	if err != nil {
		t.Fatalf("render index body: %v", err)
	}

	app.templates.status = template.Must(template.New("bad").Parse(`{{ define "index_body" }}{{ .Status.DoesNotExist }}{{ end }}`))
	now = fixedNow.Add(2 * time.Second)

	if _, rendered, err := app.refreshIndexBody(now, true); err == nil {
		t.Fatal("failing index body render returned nil error")
	} else if rendered {
		t.Fatal("failing index body render reported rendered")
	}

	current, ok := app.indexResponse.get()
	if !ok {
		t.Fatal("last good index body is missing")
	}
	if string(current.body) != string(first.body) {
		t.Fatal("failing index body render replaced last good body")
	}
	if !current.renderedAt.Equal(fixedNow) {
		t.Fatalf("last good renderedAt = %s, want %s", current.renderedAt, fixedNow)
	}

	families := scrapeMetrics(t, app)
	requireUnlabeledMetricValue(t, families["vpsfstatus_index_last_render_timestamp_seconds"], float64(fixedNow.Unix()))
	requireUnlabeledMetricValue(t, families["vpsfstatus_index_last_render_attempt_timestamp_seconds"], float64(now.Unix()))
	requireUnlabeledCounterValue(t, families["vpsfstatus_index_render_failures_total"], 1)
}
