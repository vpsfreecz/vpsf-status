package main

import (
	"fmt"
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
	about  *template.Template
}

type StatusData struct {
	Config     *config.Config
	Status     *StatusView
	RenderedAt string
	Notice     Notice
}

type AboutData struct {
	Config *config.Config
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

	about, err := template.ParseFiles(
		filepath.Join(app.config.DataDir, "templates/layout.tmpl"),
		filepath.Join(app.config.DataDir, "templates/about.tmpl"),
	)
	if err != nil {
		return err
	}

	app.templates.about = about
	return nil
}

func (app *application) handleIndex(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	view := createStatusView(app.status)

	notice, err := readNoticeFile(app.config.NoticeFile)
	if err != nil {
		log.Printf("Unable to read notice file: %+v", err)
	}

	app.setCacheControl(w, 1)

	err = app.templates.status.Execute(w, StatusData{
		Config:     app.config,
		Status:     &view,
		RenderedAt: now.Format(time.UnixDate),
		Notice:     notice,
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

	app.setCacheControl(w, 1)
	w.Header().Set("Content-Type", "application/json")

	if err := json.ExportTo(w, app.status.ToJson(now, notice)); err != nil {
		log.Printf("Error while exporting to JSON: %+v", err)
	}
}

func (app *application) handleAbout(w http.ResponseWriter, r *http.Request) {
	app.setCacheControl(w, 24*60*60)

	err := app.templates.about.Execute(w, AboutData{
		Config: app.config,
	})
	if err != nil {
		log.Printf("Template error: %+v", err)
	}
}

func (app *application) setCacheControl(w http.ResponseWriter, seconds int) {
	w.Header().Set("Cache-Control", fmt.Sprintf("max-age=%d", seconds))
}
