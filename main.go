package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

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

	checkInterval := time.Duration(cfg.CheckInterval) * time.Second

	go checkApi(systemStatus, checkInterval)
	go checkOutageReports(systemStatus, checkInterval)
	checkVpsAdminWebServices(systemStatus, checkInterval)
	pingNodes(systemStatus, checkInterval)
	pingDnsResolvers(systemStatus, checkInterval)
	checkDnsResolvers(systemStatus, checkInterval)
	checkWebServices(systemStatus, checkInterval)
	checkNameServers(systemStatus, checkInterval)

	http.HandleFunc("/", app.handleIndex)
	http.HandleFunc("/json", app.handleJson)
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
