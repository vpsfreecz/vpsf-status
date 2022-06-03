package main

import (
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/vpsfreecz/vpsf-status/config"
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
	Status     *Status
	RenderedAt string
	Notice     template.HTML
}

func (app *application) parseTemplates() error {
	status, err := template.ParseFiles(
		"templates/layout.tpl",
		"templates/status.tpl",
	)
	if err != nil {
		return err
	}

	app.templates.status = status
	return nil
}

func (app *application) handleIndex(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	app.status.CacheCounts()

	notice, err := readNoticeFile(app.config.StateDir)
	if err != nil {
		log.Printf("Unable to read notice file: %+v", err)
	}

	err = app.templates.status.Execute(w, StatusData{
		Config:     app.config,
		Status:     app.status,
		RenderedAt: now.Format(time.UnixDate),
		Notice:     template.HTML(notice),
	})

	if err != nil {
		log.Printf("Template error: %+v", err)
	}
}
