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
		`aria-label="Degraded"`,
	)
	body := rr.Body.String()
	requireStatusCountsAfter(t, body, `href="/group?kind=location&amp;id=3"`, StatusCounts{Operational: 2, Degraded: 1, Total: 3})
	requireStatusCountsAfter(t, body, `data-bs-target="#collapse-dns-3"`, StatusCounts{Degraded: 1, Total: 1})
	requireStatusCountsAfter(t, body, `href="/group?kind=services"`, StatusCounts{Operational: 2, Degraded: 1, Total: 3})
	requireStatusCountsAfter(t, body, `data-bs-target="#collapse-nameservers"`, StatusCounts{Degraded: 1, Total: 1})
}
