package main

import (
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/vpsfreecz/vpsf-status/config"
	"github.com/vpsfreecz/vpsf-status/json"
)

type application struct {
	config    *config.Config
	status    *Status
	templates htmlTemplate
}

type htmlTemplate struct {
	status *template.Template
}

type StatusData struct {
	Config     *config.Config
	Status     *StatusView
	RenderedAt string
	Notice     template.HTML
}

func (app *application) parseTemplates() error {
	status, err := template.ParseFiles(
		filepath.Join(app.config.DataDir, "templates/layout.tmpl"),
		filepath.Join(app.config.DataDir, "templates/status.tmpl"),
	)
	if err != nil {
		return err
	}

	app.templates.status = status
	return nil
}

func (app *application) handleIndex(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	view := createStatusView(app.status)

	notice, err := readNoticeFile(app.config.NoticeFile)
	if err != nil {
		log.Printf("Unable to read notice file: %+v", err)
	}

	app.setCacheControl(w)

	err = app.templates.status.Execute(w, StatusData{
		Config:     app.config,
		Status:     &view,
		RenderedAt: now.Format(time.UnixDate),
		Notice:     template.HTML(notice),
	})

	if err != nil {
		log.Printf("Template error: %+v", err)
	}
}

func (app *application) handleJson(w http.ResponseWriter, r *http.Request) {
	now := time.Now()

	notice, err := readNoticeFile(app.config.NoticeFile)
	if err != nil {
		log.Printf("Unable to read notice file: %+v", err)
	}

	app.setCacheControl(w)
	w.Header().Set("Content-Type", "application/json")

	if err := json.ExportTo(w, app.status.ToJson(now, notice)); err != nil {
		log.Printf("Error while exporting to JSON: %+v", err)
	}
}

func (app *application) setCacheControl(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "max-age=1")
}
