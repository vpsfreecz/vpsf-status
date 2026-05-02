package main

import (
	"net/http"
	"testing"
)

func TestRoutesServeIndexResolverDegradedState(t *testing.T) {
	app, st, _ := newTestApplication(t)
	setOperationalFixture(st)

	setResolverState(st.Exporter.dnsResolverPing, st.Exporter.dnsResolverLookup, st.LocationMap["Praha"].DnsResolverList[0], 40, true)
	setResolverState(st.Exporter.nameServerPing, st.Exporter.nameServerLookup, st.Services.NameServer[0], 40, true)

	rr := getThroughRoutes(t, app, "/")
	requireStatus(t, rr, http.StatusOK)

	requireContains(
		t,
		rr.Body.String(),
		"Praha 3/3",
		"DNS Resolvers 1/1",
		"Services 3/3",
		"Name Servers 1/1",
		`aria-label="Degraded"`,
	)
}
