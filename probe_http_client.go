package main

import (
	"net/http"
	"time"

	vpsadminclient "github.com/vpsfreecz/vpsadmin-go-client/client"
)

func newProbeHTTPClient(checkTimeout time.Duration) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	// A stale persistent connection can survive a probe-host network flap and
	// keep reporting recovered services as unreachable.
	transport.DisableKeepAlives = true

	client := &http.Client{
		Transport: transport,
	}

	if checkTimeout > 0 {
		client.Timeout = checkTimeout
	}

	return client
}

func newVpsAdminClient(apiUrl string, checkTimeout time.Duration) *vpsadminclient.Client {
	apiClient := vpsadminclient.New(apiUrl)
	apiClient.SetHTTPClient(newProbeHTTPClient(checkTimeout))

	return apiClient
}
