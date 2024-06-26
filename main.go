package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/vpsfreecz/vpsf-status/config"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("Usage: %s <config file>", os.Args[0])
	}

	cfg, err := config.ParseConfig(os.Args[1])
	if err != nil {
		log.Fatalf("Unable to parse config: %+v", err)
	}

	systemStatus := openConfig(cfg)

	app := application{config: cfg, status: systemStatus}

	if err := app.parseTemplates(); err != nil {
		log.Fatalf("Unable to parse template: %+v", err)
	}

	go systemStatus.initialize(cfg)

	http.HandleFunc("/", app.handleIndex)
	http.HandleFunc("/json", app.handleJson)
	http.Handle("/metrics", systemStatus.Exporter.httpHandler())
	http.HandleFunc("/about", app.handleAbout)
	http.Handle(
		"/static/",
		http.StripPrefix(
			"/static/",
			http.FileServer(http.Dir(filepath.Join(cfg.DataDir, "public"))),
		),
	)

	fmt.Printf("Starting server...\n")
	if err := http.ListenAndServe(cfg.ListenAddress, nil); err != nil {
		log.Fatal(err)
	}
}
