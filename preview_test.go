//go:build preview

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func previewApplicationForTest(t *testing.T, scenario string) *application {
	t.Helper()
	app, err := newPreviewApplication(".", t.TempDir(), scenario)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { app.status.History.Close() })
	return app
}

func TestPreviewRoutes(t *testing.T) {
	app := previewApplicationForTest(t, "mixed")
	handler := previewHandler(app, "mixed")
	for _, lang := range []string{"en", "cs"} {
		for _, path := range []string{"/?", "/about?", "/entity?kind=node&id=node01.preview.example.invalid&", "/entity?kind=node&id=node01.preview.example.invalid&probe_page=2&", "/group?kind=location&id=1&", "/group?kind=services&", "/group?kind=vpsadmin&"} {
			t.Run(lang+path, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodGet, path+"lang="+lang, nil)
				req.Header.Set("Accept-Encoding", "gzip")
				resp := httptest.NewRecorder()
				handler.ServeHTTP(resp, req)
				if resp.Code != http.StatusOK {
					t.Fatalf("status %d: %s", resp.Code, resp.Body.String())
				}
				for _, want := range []string{`<html lang="` + lang + `">`, "Preview with sample data / Ukázka s testovacími daty"} {
					if !strings.Contains(resp.Body.String(), want) {
						t.Errorf("missing %q", want)
					}
				}
				if resp.Header().Get("Content-Encoding") != "" {
					t.Fatal("banner response unexpectedly compressed")
				}
			})
		}
	}

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/json", nil))
	if resp.Code != http.StatusOK || !json.Valid(resp.Body.Bytes()) || strings.Contains(resp.Body.String(), "<aside") {
		t.Fatalf("invalid preview JSON: %d %s", resp.Code, resp.Body.String())
	}
	if app.status.requestIndexRender != nil {
		t.Fatal("preview started the background index renderer")
	}
}

func TestPreviewScenarios(t *testing.T) {
	for _, scenario := range []string{"mixed", "healthy", "loading", "fetch-failure"} {
		t.Run(scenario, func(t *testing.T) {
			app := previewApplicationForTest(t, scenario)
			for _, lang := range []string{"en", "cs"} {
				resp := httptest.NewRecorder()
				previewHandler(app, scenario).ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/?lang="+lang, nil))
				body := resp.Body.String()
				if resp.Code != http.StatusOK || !strings.Contains(body, "Preview with sample data / Ukázka s testovacími daty") {
					t.Fatalf("unlabelled or failed %s page: %d", lang, resp.Code)
				}
				loc, _ := app.locales.localeForCode(lang, nil)
				switch scenario {
				case "mixed":
					if !strings.Contains(body, "node03-with-a-long-hostname") || !strings.Contains(body, "Synthetic security advisory") {
						t.Error("missing long-label/advisory fixture")
					}
				case "healthy":
					view := createStatusView(app.status, previewNow)
					if !view.VpsAdmin.IsOperational() || !view.Services.IsOperational() {
						t.Error("healthy fixture is not operational")
					}
					for _, location := range view.Locations {
						if !location.IsOperational() {
							t.Errorf("healthy location %s is not operational", location.Label)
						}
					}
				case "loading":
					if !strings.Contains(body, loc.T("loading.title")) || app.status.Initialized {
						t.Error("loading fixture did not render loading page")
					}
				case "fetch-failure":
					for _, key := range []string{"outages.unable_fetch", "security.unable_fetch"} {
						if !strings.Contains(body, loc.T(key)) {
							t.Errorf("missing fetch-failure example %s", key)
						}
					}
				}
			}
		})
	}
}

func TestPreviewHistoryIsolationAndPagination(t *testing.T) {
	mixed := previewApplicationForTest(t, "mixed")
	healthy := previewApplicationForTest(t, "healthy")
	if mixed.config.StateDir == healthy.config.StateDir || !mixed.currentTime().Equal(previewNow) {
		t.Fatal("fixtures share state or do not use the frozen clock")
	}
	for _, app := range []*application{mixed, healthy} {
		if _, err := os.Stat(filepath.Join(app.config.StateDir, historyDBFilename)); err != nil {
			t.Fatal(err)
		}
	}
	entity, ok := createEntityDetailView(mixed.status, historyEntityNode, "node01.preview.example.invalid", previewNow, 2)
	if !ok || entity.EventPagination.TotalPages < 2 || entity.EventPagination.Page != 2 || len(entity.Events) == 0 {
		t.Fatalf("missing second event page: %+v", entity.EventPagination)
	}
	healthyEntity, ok := createEntityDetailView(healthy.status, historyEntityNode, "node01.preview.example.invalid", previewNow, 1)
	if !ok || healthyEntity.EventPagination.Total != 0 {
		t.Fatalf("mixed history leaked into healthy fixture: %+v", healthyEntity.EventPagination)
	}
	if _, err := newPreviewApplication(".", t.TempDir(), "unknown"); err == nil {
		t.Fatal("unknown scenario was accepted")
	}
}
