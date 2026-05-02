package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
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
	history, err := openHistoryStore(cfg.HistoryDir)
	if err != nil {
		log.Fatalf("Unable to initialize history storage: %+v", err)
	}
	systemStatus.History = history
	if err := recordConfiguredEntitySnapshots(systemStatus, time.Now()); err != nil {
		log.Printf("Unable to store entity snapshots: %+v", err)
	}

	app := application{config: cfg, status: systemStatus}

	if err := app.parseTemplates(); err != nil {
		log.Fatalf("Unable to parse template: %+v", err)
	}

	go systemStatus.initialize(cfg)

	fmt.Printf("Starting server...\n")
	if err := http.ListenAndServe(cfg.ListenAddress, app.routes()); err != nil {
		log.Fatal(err)
	}
}
