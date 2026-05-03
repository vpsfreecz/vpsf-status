package main

import (
	"bytes"
	"html/template"
	"log"
	"net/http"
	"sync"
	"time"
)

const indexRenderInterval = time.Second

type preRenderedIndex struct {
	mu       sync.RWMutex
	renderMu sync.Mutex
	response cachedResponse
	ready    bool
}

func newPreRenderedIndex() *preRenderedIndex {
	return &preRenderedIndex{}
}

func (c *preRenderedIndex) get() (cachedResponse, bool) {
	if c == nil {
		return cachedResponse{}, false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.ready {
		return cachedResponse{}, false
	}

	return c.response, true
}

func (c *preRenderedIndex) set(response cachedResponse) {
	if c == nil {
		return
	}

	c.mu.Lock()
	c.response = response
	c.ready = true
	c.mu.Unlock()
}

func (app *application) handleIndex(w http.ResponseWriter, r *http.Request) {
	app.ensureCaches()

	if entry, ok := app.indexResponse.get(); ok {
		writeResponse(w, r, &entry)
		return
	}

	entry, err := app.refreshIndexResponse(app.currentTime())
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	writeResponse(w, r, &entry)
}

func (app *application) startIndexRenderer() {
	app.ensureCaches()

	if _, err := app.refreshIndexResponse(app.currentTime()); err != nil {
		log.Printf("Unable to pre-render index page: %+v", err)
	}

	go func() {
		ticker := time.NewTicker(indexRenderInterval)
		defer ticker.Stop()

		for range ticker.C {
			app.refreshIndexResponseIfIdle(app.currentTime())
		}
	}()
}

func (app *application) refreshIndexResponse(now time.Time) (cachedResponse, error) {
	app.ensureCaches()

	app.indexResponse.renderMu.Lock()
	defer app.indexResponse.renderMu.Unlock()

	return app.renderIndexResponseLocked(now)
}

func (app *application) refreshIndexResponseIfIdle(now time.Time) {
	app.ensureCaches()

	if !app.indexResponse.renderMu.TryLock() {
		return
	}
	defer app.indexResponse.renderMu.Unlock()

	if _, err := app.renderIndexResponseLocked(now); err != nil {
		log.Printf("Unable to pre-render index page: %+v", err)
	}
}

func (app *application) renderIndexResponseLocked(now time.Time) (cachedResponse, error) {
	payload, err := app.buildIndexPayload(now)
	if err != nil {
		return cachedResponse{}, err
	}
	if payload.statusCode == 0 {
		payload.statusCode = http.StatusOK
	}

	entry := newCachedResponse(payload, now)
	app.indexResponse.set(entry)
	app.status.Exporter.indexLastRender.Set(float64(now.Unix()))
	return entry, nil
}

func (app *application) buildIndexPayload(now time.Time) (responsePayload, error) {
	notice, err := app.readNotice(now)
	if err != nil {
		log.Printf("Unable to read notice file: %+v", err)
	}

	data := StatusData{
		Config:     app.config,
		RenderedAt: now.Format(time.UnixDate),
		Notice:     notice,
	}

	if !app.status.Initialized {
		return app.executeIndexTemplate(app.templates.loading, data)
	}

	view := createStatusView(app.status, now)
	data.Status = &view
	return app.executeIndexTemplate(app.templates.status, data)
}

func (app *application) executeIndexTemplate(tpl *template.Template, data StatusData) (responsePayload, error) {
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
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
}
