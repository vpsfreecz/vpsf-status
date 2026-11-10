package main

import (
	"net/http"
	"regexp"
	"strings"
	"testing"
)

// Counts stay plain text and inherit the same palette token as their history bars.
// Keep this source invariant separate from browser checks of the actual cascade.
func TestStatusCountColorsMatchHistoryWithoutBadges(t *testing.T) {
	app, _, _ := newTestApplication(t)
	rr := getThroughRoutes(t, app, "/static/status.css")
	requireStatus(t, rr, http.StatusOK)
	css := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(rr.Body.String(), "")
	space := regexp.MustCompile(`[[:space:]]+`)
	normalize := func(declaration string) string {
		return strings.TrimSuffix(space.ReplaceAllString(declaration, ""), ";")
	}
	rules := make(map[string][]string)
	for _, match := range regexp.MustCompile(`([^{}]+)\{([^{}]*)\}`).FindAllStringSubmatch(css, -1) {
		selector := strings.TrimSpace(match[1])
		rules[selector] = append(rules[selector], strings.TrimSpace(match[2]))
		if strings.Contains(selector, ".status-counts") && selector != ".status-counts > .text-success" && selector != ".status-counts > .text-warning" && selector != ".status-counts > .text-danger" {
			t.Errorf("unexpected count override %q: %s", selector, match[2])
		}
	}
	for _, status := range []struct{ count, history string }{
		{"success", "ok"}, {"warning", "maintenance"}, {"danger", "outage"},
	} {
		t.Run(status.count, func(t *testing.T) {
			token := "var(--history-" + status.history + ")"
			count := rules[".status-counts > .text-"+status.count]
			if len(count) != 1 || normalize(count[0]) != "color:"+token+"!important" {
				t.Fatalf("plain count foreground must use only %s; got %q", token, count)
			}
			bar := rules[".history-day-"+status.history]
			if len(bar) != 1 || normalize(bar[0]) != "background:"+token {
				t.Fatalf("history background must share %s; got %q", token, bar)
			}
			for _, theme := range []string{":root", `:root[data-theme="dark"]`, `:root:not([data-theme="light"])`} {
				if len(rules[theme]) != 1 || !strings.Contains(rules[theme][0], "--history-"+status.history+":") {
					t.Errorf("missing shared history token in light/manual-dark/system-dark context %s", theme)
				}
			}
		})
	}
}

// Exercise the actual route shells, including the cached overview and about page.
func TestHTMLRoutesShareLocalizedDisplayControls(t *testing.T) {
	for _, locale := range []struct {
		code, settings, label, system, light, dark string
	}{
		{"en", "Settings", "Theme", "System", "Light", "Dark"},
		{"cs", "Nastavení", "Vzhled", "Podle systému", "Světlý", "Tmavý"},
	} {
		t.Run(locale.code, func(t *testing.T) {
			app, st, _ := newTestApplication(t)
			setOperationalFixture(st)
			for _, route := range []string{"/?", "/about?", "/entity?kind=node&id=node1.prg&", "/group?kind=location&id=3&"} {
				t.Run(route, func(t *testing.T) {
					rr := getThroughRoutes(t, app, route+"lang="+locale.code)
					requireStatus(t, rr, http.StatusOK)
					body := rr.Body.String()
					if got := strings.Count(body, `class="settings-menu"`); got != 1 {
						t.Fatalf("settings trigger count = %d, want one shared header", got)
					}
					requireContains(t, body,
						`<summary aria-label="`+locale.settings+`">`,
						`<legend>`+locale.label+`</legend>`,
						`id="theme-system" title="`+locale.system+`"`,
						`id="theme-light" title="`+locale.light+`"`,
						`id="theme-dark" title="`+locale.dark+`"`,
						`/static/status.css?v=10`, `/static/theme.js?v=4`,
						`class="language-options" role="group" aria-labelledby="language-label"`,
					)
					if got := strings.Count(body, `class="language-option"`); got != 2 {
						t.Fatalf("language button count = %d, want two native links", got)
					}
					requireNotContains(t, body, `id="language-select"`, `class="settings-fallback"`)
				})
			}
		})
	}
}

func TestLoadingRouteIncludesDisplayControls(t *testing.T) {
	app, _, _ := newTestApplication(t)
	rr := getThroughRoutes(t, app, "/?lang=cs")
	requireStatus(t, rr, http.StatusOK)
	requireContains(t, rr.Body.String(), `id="theme-system"`, `title="Podle systému"`)
}

func TestIncidentTablesKeepLocalizedFieldLabels(t *testing.T) {
	app, st, _ := newTestApplication(t)
	setOperationalFixture(st)
	addOutageFixture(st)
	addSecurityAdvisoryFixture(st)

	rr := getThroughRoutes(t, app, "/?lang=cs")
	requireStatus(t, rr, http.StatusOK)
	requireContains(t, rr.Body.String(),
		`<th scope="col" role="columnheader">Důvod</th>`,
		`class="cell-label" aria-hidden="true">Důvod</span>`,
		`class="cell-label" aria-hidden="true">CVE</span>`,
		"Router replacement", "CVE-2026-2001",
	)
}

func TestSettingsLanguageButtonsPreserveSwitchURLs(t *testing.T) {
	app, st, _ := newTestApplication(t)
	setOperationalFixture(st)
	for _, route := range []struct {
		name, path, english, czech string
	}{
		{"entity", "/entity?kind=node&id=node1.prg&probe_page=2&scenario=mixed&lang=cs", "/entity?id=node1.prg&amp;kind=node&amp;lang=en&amp;probe_page=2&amp;scenario=mixed", "/entity?id=node1.prg&amp;kind=node&amp;lang=cs&amp;probe_page=2&amp;scenario=mixed"},
		{"group", "/group?kind=location&id=3&probe_page=2&scenario=mixed&lang=cs", "/group?id=3&amp;kind=location&amp;lang=en&amp;probe_page=2&amp;scenario=mixed", "/group?id=3&amp;kind=location&amp;lang=cs&amp;probe_page=2&amp;scenario=mixed"},
		{"about", "/about?scenario=mixed&lang=cs", "/?lang=en", "/?lang=cs"},
	} {
		t.Run(route.name, func(t *testing.T) {
			rr := getThroughRoutes(t, app, route.path)
			requireStatus(t, rr, http.StatusOK)
			requireContains(t, rr.Body.String(),
				`<a class="language-option" href="`+route.english+`" title="Přepnout na English">English</a>`,
				`<a class="language-option" href="`+route.czech+`" title="Přepnout na Česky" aria-current="true">Česky</a>`,
				`class="settings-enhanced" hidden`,
			)
		})
	}
}
