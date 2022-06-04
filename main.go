package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

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

	go checkApi(systemStatus)
	checkVpsAdminWebServices(systemStatus)
	pingNodes(systemStatus)
	pingDnsResolvers(systemStatus)
	checkDnsResolvers(systemStatus)
	checkWebServices(systemStatus)

	http.HandleFunc("/", app.handleIndex)
	http.HandleFunc("/json", app.handleJson)

	fmt.Printf("Starting server...\n")
	if err := http.ListenAndServe(cfg.ListenAddress, nil); err != nil {
		log.Fatal(err)
	}
}
