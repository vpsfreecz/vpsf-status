package main

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"github.com/vpsfreecz/vpsf-status/config"
	"github.com/vpsfreecz/vpsf-status/json"
)

type application struct {
	config    *config.Config
	status    *Status
	templates htmlTemplate
	locales   *localeCatalog
	now       func() time.Time

	responseCache  *responseCache
	noticeCache    *noticeCache
	indexResponse  *preRenderedIndex
	aboutResponses map[string]cachedResponse
}

type htmlTemplate struct {
	loading    *template.Template
	status     *template.Template
	indexShell *template.Template
	entity     *template.Template
	about      *template.Template
}

type StatusData struct {
	Config          *config.Config
	Locale          *pageLocale
	Status          *StatusView
	RenderedAt      string
	Notice          Notice
	NoticeUpdatedAt string
}

type IndexShellData struct {
	Config     *config.Config
	Locale     *pageLocale
	RenderedAt string
	Body       template.HTML
}

type AboutData struct {
	Config *config.Config
	Locale *pageLocale
}

type EntityData struct {
	Config     *config.Config
	Locale     *pageLocale
	Entity     EntityDetailView
	RenderedAt string
}

func (app *application) parseTemplates() error {
	app.ensureLocales()

	if tpl, err := app.parseTemplateWithLayout("loading.tmpl"); err == nil {
		app.templates.loading = tpl
	} else {
		return err
	}

	if tpl, err := app.parseTemplateWithLayout("status.tmpl"); err == nil {
		app.templates.status = tpl
	} else {
		return err
	}

	if tpl, err := app.parseTemplateWithLayout("index_shell.tmpl"); err == nil {
		app.templates.indexShell = tpl
	} else {
		return err
	}

	if tpl, err := app.parseTemplateWithLayout("entity.tmpl"); err == nil {
		app.templates.entity = tpl
	} else {
		return err
	}

	if tpl, err := app.parseTemplateWithLayout("about.tmpl"); err == nil {
		app.templates.about = tpl
	} else {
		return err
	}

	return app.renderAboutResponses()
}

func (app *application) parseTemplateWithLayout(name string) (*template.Template, error) {
	tpl, err := template.New("layout.tmpl").Funcs(template.FuncMap{
		"dict": templateDict,
	}).ParseFiles(
		filepath.Join(app.config.DataDir, "templates/layout.tmpl"),
		filepath.Join(app.config.DataDir, "templates/index_header.tmpl"),
		filepath.Join(app.config.DataDir, "templates/history_bar.tmpl"),
		filepath.Join(app.config.DataDir, fmt.Sprintf("templates/%s", name)),
	)
	if err != nil {
		return nil, err
	}

	return tpl, nil
}

func (app *application) handleEntity(w http.ResponseWriter, r *http.Request) {
	loc, redirected := app.resolveHTMLLocale(w, r)
	if redirected {
		return
	}

	app.serveCachedResponse(w, r, routeCacheKey(r), func(now time.Time) (responsePayload, error) {
		entity, ok := createEntityDetailViewForLocale(app.status, r.URL.Query().Get("kind"), r.URL.Query().Get("id"), now, probeLogPageFromRequest(r), loc)
		if !ok {
			return notFoundPayload(), nil
		}

		var buf bytes.Buffer
		err := app.templates.entity.Execute(&buf, EntityData{
			Config:     app.config,
			Locale:     loc,
			Entity:     entity,
			RenderedAt: formatGeneratedAt(now, loc),
		})
		if err != nil {
			log.Printf("Template error: %+v", err)
			return responsePayload{}, err
		}

		return responsePayload{
			statusCode:   http.StatusOK,
			contentType:  "text/html; charset=utf-8",
			cacheControl: "max-age=1",
			body:         buf.Bytes(),
			cacheFor:     dynamicCacheTTL,
		}, nil
	})
}

func (app *application) handleGroup(w http.ResponseWriter, r *http.Request) {
	loc, redirected := app.resolveHTMLLocale(w, r)
	if redirected {
		return
	}

	app.serveCachedResponse(w, r, routeCacheKey(r), func(now time.Time) (responsePayload, error) {
		group, ok := createGroupDetailViewForLocale(app.status, r.URL.Query().Get("kind"), r.URL.Query().Get("id"), now, probeLogPageFromRequest(r), loc)
		if !ok {
			return notFoundPayload(), nil
		}

		var buf bytes.Buffer
		err := app.templates.entity.Execute(&buf, EntityData{
			Config:     app.config,
			Locale:     loc,
			Entity:     group,
			RenderedAt: formatGeneratedAt(now, loc),
		})
		if err != nil {
			log.Printf("Template error: %+v", err)
			return responsePayload{}, err
		}

		return responsePayload{
			statusCode:   http.StatusOK,
			contentType:  "text/html; charset=utf-8",
			cacheControl: "max-age=1",
			body:         buf.Bytes(),
			cacheFor:     dynamicCacheTTL,
		}, nil
	})
}

func (app *application) handleJson(w http.ResponseWriter, r *http.Request) {
	app.serveCachedResponse(w, r, routeCacheKey(r), func(now time.Time) (responsePayload, error) {
		notice, err := app.readNotice(now)
		if err != nil {
			log.Printf("Unable to read notice file: %+v", err)
		}

		var buf bytes.Buffer
		if err := json.ExportTo(&buf, app.status.ToJson(now, notice)); err != nil {
			log.Printf("Error while exporting to JSON: %+v", err)
			return responsePayload{}, err
		}

		return responsePayload{
			statusCode:   http.StatusOK,
			contentType:  "application/json",
			cacheControl: "max-age=1",
			body:         buf.Bytes(),
			cacheFor:     dynamicCacheTTL,
		}, nil
	})
}

func (app *application) handleAbout(w http.ResponseWriter, r *http.Request) {
	loc, redirected := app.resolveHTMLLocale(w, r)
	if redirected {
		return
	}

	app.setCacheControl(w, 24*60*60)
	if response, ok := app.aboutResponses[loc.Code]; ok {
		writeResponse(w, r, &response)
		return
	}

	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

func (app *application) setCacheControl(w http.ResponseWriter, seconds int) {
	w.Header().Set("Cache-Control", fmt.Sprintf("max-age=%d", seconds))
}

func (app *application) routes() http.Handler {
	app.ensureCaches()

	mux := http.NewServeMux()
	mux.HandleFunc("/", app.handleIndex)
	mux.HandleFunc("/entity", app.handleEntity)
	mux.HandleFunc("/group", app.handleGroup)
	mux.HandleFunc("/json", app.handleJson)
	mux.Handle("/metrics", app.status.Exporter.httpHandler())
	mux.HandleFunc("/about", app.handleAbout)
	mux.Handle(
		"/static/",
		staticCacheControl(
			http.StripPrefix(
				"/static/",
				http.FileServer(http.Dir(filepath.Join(app.config.DataDir, "public"))),
			),
		),
	)
	return mux
}

func (app *application) currentTime() time.Time {
	if app.now != nil {
		return app.now()
	}

	return time.Now()
}

func probeLogPageFromRequest(r *http.Request) int {
	if r == nil || r.URL == nil {
		return 1
	}

	page, err := strconv.Atoi(r.URL.Query().Get(probeLogPageParam))
	if err != nil || page < 1 {
		return 1
	}
	return page
}

func (app *application) ensureCaches() {
	if app.responseCache == nil {
		app.responseCache = newResponseCache()
	}
	if app.noticeCache == nil {
		app.noticeCache = newNoticeCache()
	}
	if app.indexResponse == nil {
		app.indexResponse = newPreRenderedIndex()
	}
	app.ensureLocales()
}

func (app *application) readNotice(now time.Time) (Notice, error) {
	app.ensureCaches()
	return app.noticeCache.read(app.config.NoticeFile, now)
}

func (app *application) renderAboutResponses() error {
	app.ensureLocales()
	app.aboutResponses = make(map[string]cachedResponse, len(app.locales.languages))

	for _, info := range app.locales.languages {
		loc, _ := app.locales.localeForCode(info.Code, nil)
		var buf bytes.Buffer
		if err := app.templates.about.Execute(&buf, AboutData{Config: app.config, Locale: loc}); err != nil {
			return err
		}

		response := cachedResponse{
			statusCode:  http.StatusOK,
			contentType: "text/html; charset=utf-8",
			body:        append([]byte(nil), buf.Bytes()...),
		}
		response.gzipBody = gzipBytes(response.body)
		app.aboutResponses[info.Code] = response
	}
	return nil
}

func staticCacheControl(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=86400")
		next.ServeHTTP(w, r)
	})
}

func notFoundPayload() responsePayload {
	return responsePayload{
		statusCode:  http.StatusNotFound,
		contentType: "text/plain; charset=utf-8",
		body:        []byte("404 page not found\n"),
	}
}
