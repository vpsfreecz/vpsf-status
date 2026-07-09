package main

import (
	"bytes"
	"fmt"
	"hash"
	"hash/fnv"
	"html/template"
	"log"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"
)

const (
	indexRenderMinInterval = time.Second
	indexRenderKeepalive   = 4 * time.Minute
	indexBodyPlaceholder   = "__VPSF_STATUS_INDEX_BODY__"
)

type preRenderedIndexBody struct {
	body       []byte
	signature  string
	renderedAt time.Time
}

type indexShellResponse struct {
	statusCode   int
	contentType  string
	cacheControl string
	prefix       []byte
	suffix       []byte
}

type preRenderedIndex struct {
	mu             sync.RWMutex
	renderMu       sync.Mutex
	bodies         map[string]preRenderedIndexBody
	lastAttempt    time.Time
	renderRequests chan struct{}
}

func newPreRenderedIndex() *preRenderedIndex {
	return &preRenderedIndex{
		bodies:         make(map[string]preRenderedIndexBody),
		renderRequests: make(chan struct{}, 1),
	}
}

func (c *preRenderedIndex) get(lang ...string) (preRenderedIndexBody, bool) {
	if c == nil {
		return preRenderedIndexBody{}, false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	code := defaultLang
	if len(lang) > 0 && lang[0] != "" {
		code = lang[0]
	}
	body, ok := c.bodies[code]
	return body, ok
}

func (c *preRenderedIndex) set(lang string, body preRenderedIndexBody) {
	if c == nil {
		return
	}

	c.mu.Lock()
	c.bodies[lang] = body
	c.mu.Unlock()
}

func (c *preRenderedIndex) setLastAttempt(at time.Time) {
	if c == nil {
		return
	}

	c.mu.Lock()
	c.lastAttempt = at
	c.mu.Unlock()
}

func (c *preRenderedIndex) nextRenderDelay(now time.Time) time.Duration {
	if c == nil {
		return 0
	}

	c.mu.RLock()
	lastAttempt := c.lastAttempt
	c.mu.RUnlock()

	next := lastAttempt.Add(indexRenderMinInterval)
	if lastAttempt.IsZero() || !now.Before(next) {
		return 0
	}
	return next.Sub(now)
}

func (c *preRenderedIndex) canSkip(lang string, signature string, now time.Time) (preRenderedIndexBody, bool) {
	body, ok := c.get(lang)
	if !ok {
		return preRenderedIndexBody{}, false
	}
	if body.signature != signature {
		return preRenderedIndexBody{}, false
	}
	if body.renderedAt.IsZero() || !now.Before(body.renderedAt.Add(indexRenderKeepalive)) {
		return preRenderedIndexBody{}, false
	}
	return body, true
}

func (app *application) handleIndex(w http.ResponseWriter, r *http.Request) {
	loc, redirected := app.resolveHTMLLocale(w, r)
	if redirected {
		return
	}

	app.ensureCaches()
	now := app.currentTime()

	body, ok := app.indexResponse.get(loc.Code)
	if !ok {
		var err error
		body, _, err = app.refreshIndexBody(now, true, loc)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}

	shell, err := app.renderIndexShell(now, loc)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	writeIndexResponse(w, r, shell, body.body)
}

func (app *application) startIndexRenderer() {
	app.ensureCaches()
	if app.status != nil {
		app.status.requestIndexRender = app.requestIndexRender
	}

	if err := app.refreshIndexBodies(app.currentTime(), true); err != nil {
		log.Printf("Unable to pre-render index page: %+v", err)
	}

	go app.runIndexRenderer()
}

func (app *application) requestIndexRender() {
	app.ensureCaches()

	select {
	case app.indexResponse.renderRequests <- struct{}{}:
	default:
	}
}

func (app *application) runIndexRenderer() {
	for range app.indexResponse.renderRequests {
		app.drainIndexRenderRequests()

		now := app.currentTime()
		if delay := app.indexResponse.nextRenderDelay(now); delay > 0 {
			time.Sleep(delay)
			app.drainIndexRenderRequests()
			now = app.currentTime()
		}

		app.refreshIndexResponseIfIdle(now)
	}
}

func (app *application) drainIndexRenderRequests() {
	for {
		select {
		case <-app.indexResponse.renderRequests:
		default:
			return
		}
	}
}

func (app *application) refreshIndexResponse(now time.Time) (cachedResponse, error) {
	loc := defaultPageLocale()
	body, _, err := app.refreshIndexBody(now, true, loc)
	if err != nil {
		return cachedResponse{}, err
	}

	shell, err := app.renderIndexShell(now, loc)
	if err != nil {
		return cachedResponse{}, err
	}
	return shell.cachedResponse(body.body), nil
}

func (app *application) refreshIndexResponseIfIdle(now time.Time) {
	if err := app.refreshIndexBodiesIfIdle(now, false); err != nil {
		log.Printf("Unable to pre-render index page: %+v", err)
	}
}

func (app *application) refreshIndexBodies(now time.Time, force bool) error {
	app.ensureCaches()

	app.indexResponse.renderMu.Lock()
	defer app.indexResponse.renderMu.Unlock()

	for _, info := range app.locales.languages {
		loc, _ := app.locales.localeForCode(info.Code, nil)
		if _, _, err := app.renderIndexBodyLocked(now, force, loc); err != nil {
			return err
		}
	}
	return nil
}

func (app *application) refreshIndexBodiesIfIdle(now time.Time, force bool) error {
	app.ensureCaches()

	if !app.indexResponse.renderMu.TryLock() {
		return nil
	}
	defer app.indexResponse.renderMu.Unlock()

	for _, info := range app.locales.languages {
		loc, _ := app.locales.localeForCode(info.Code, nil)
		if _, _, err := app.renderIndexBodyLocked(now, force, loc); err != nil {
			return err
		}
	}
	return nil
}

func (app *application) refreshIndexBody(now time.Time, force bool, locales ...*pageLocale) (preRenderedIndexBody, bool, error) {
	app.ensureCaches()
	loc := defaultPageLocale()
	if len(locales) > 0 && locales[0] != nil {
		loc = locales[0]
	}

	app.indexResponse.renderMu.Lock()
	defer app.indexResponse.renderMu.Unlock()

	return app.renderIndexBodyLocked(now, force, loc)
}

func (app *application) renderIndexBodyLocked(now time.Time, force bool, loc *pageLocale) (preRenderedIndexBody, bool, error) {
	app.status.Exporter.indexLastAttempt.Set(float64(now.Unix()))
	app.indexResponse.setLastAttempt(now)

	notice, err := app.readNotice(now)
	if err != nil {
		log.Printf("Unable to read notice file: %+v", err)
	}

	signature := app.indexRenderSignature(notice)
	if !force {
		if body, ok := app.indexResponse.canSkip(loc.Code, signature, now); ok {
			app.status.Exporter.indexRenderSkips.Inc()
			return body, false, nil
		}
	}

	startedAt := time.Now()
	body, err := app.buildIndexBody(now, notice, loc)
	if err != nil {
		app.status.Exporter.indexRenderFailures.Inc()
		return preRenderedIndexBody{}, false, err
	}

	completedAt := app.currentTime()
	entry := preRenderedIndexBody{
		body:       append([]byte(nil), body...),
		signature:  signature,
		renderedAt: completedAt,
	}
	app.indexResponse.set(loc.Code, entry)
	app.status.Exporter.indexLastRender.Set(float64(completedAt.Unix()))
	app.status.Exporter.indexRenderDuration.Set(time.Since(startedAt).Seconds())
	return entry, true, nil
}

func (app *application) buildIndexBody(now time.Time, notice Notice, loc *pageLocale) ([]byte, error) {
	data := StatusData{
		Config:          app.config,
		Locale:          loc,
		Notice:          notice,
		NoticeUpdatedAt: formatNoticeTimestamp(notice.UpdatedAt, loc),
	}

	if !app.status.Initialized {
		return app.executeIndexBodyTemplate(app.templates.loading, data)
	}

	view := createStatusViewForLocale(app.status, now, loc)
	data.Status = &view
	return app.executeIndexBodyTemplate(app.templates.status, data)
}

func (app *application) executeIndexBodyTemplate(tpl *template.Template, data StatusData) ([]byte, error) {
	var buf bytes.Buffer
	if err := tpl.ExecuteTemplate(&buf, "index_body", data); err != nil {
		log.Printf("Template error: %+v", err)
		return nil, err
	}

	return buf.Bytes(), nil
}

func (app *application) renderIndexShell(now time.Time, loc *pageLocale) (indexShellResponse, error) {
	var buf bytes.Buffer
	err := app.templates.indexShell.Execute(&buf, IndexShellData{
		Config:     app.config,
		Locale:     loc,
		RenderedAt: formatGeneratedAt(now, loc),
		Body:       template.HTML(indexBodyPlaceholder),
	})
	if err != nil {
		log.Printf("Template error: %+v", err)
		return indexShellResponse{}, err
	}

	rendered := buf.Bytes()
	placeholder := []byte(indexBodyPlaceholder)
	i := bytes.Index(rendered, placeholder)
	if i < 0 {
		return indexShellResponse{}, fmt.Errorf("index shell placeholder not found")
	}

	return indexShellResponse{
		statusCode:   http.StatusOK,
		contentType:  "text/html; charset=utf-8",
		cacheControl: "max-age=1",
		prefix:       append([]byte(nil), rendered[:i]...),
		suffix:       append([]byte(nil), rendered[i+len(placeholder):]...),
	}, nil
}

func (shell indexShellResponse) cachedResponse(body []byte) cachedResponse {
	fullBody := make([]byte, 0, len(shell.prefix)+len(body)+len(shell.suffix))
	fullBody = append(fullBody, shell.prefix...)
	fullBody = append(fullBody, body...)
	fullBody = append(fullBody, shell.suffix...)

	return cachedResponse{
		statusCode:   shell.statusCode,
		contentType:  shell.contentType,
		cacheControl: shell.cacheControl,
		body:         fullBody,
	}
}

func writeIndexResponse(w http.ResponseWriter, r *http.Request, shell indexShellResponse, body []byte) {
	if shell.contentType != "" {
		w.Header().Set("Content-Type", shell.contentType)
	}
	if shell.cacheControl != "" {
		w.Header().Set("Cache-Control", shell.cacheControl)
	}
	w.Header().Add("Vary", "Accept-Encoding")

	if requestAcceptsGzip(r) {
		w.Header().Set("Content-Encoding", "gzip")
		w.WriteHeader(shell.statusCode)
		if r == nil || r.Method != http.MethodHead {
			_ = writeGzipChunks(w, shell.prefix, body, shell.suffix)
		}
		return
	}

	w.WriteHeader(shell.statusCode)
	if r == nil || r.Method != http.MethodHead {
		_, _ = w.Write(shell.prefix)
		_, _ = w.Write(body)
		_, _ = w.Write(shell.suffix)
	}
}

func (app *application) indexRenderSignature(notice Notice) string {
	h := fnv.New64a()
	st := app.status

	if st == nil {
		writeIndexSignature(h, "status", "nil")
		return strconv.FormatUint(h.Sum64(), 16)
	}

	writeIndexSignature(
		h,
		"status",
		st.Initialized,
		st.HistoryDays,
		st.indexHistoryVersion.Load(),
	)
	writeNoticeSignature(h, notice)
	writeVpsAdminSignature(h, st.VpsAdmin)
	writeVpsAdminLocationsSignature(h, st.VpsAdminLocations)
	writeOutageReportsSignature(h, st.OutageReports)
	writeSecurityAdvisoriesSignature(h, st.SecurityAdvisories)

	for _, loc := range st.LocationList {
		writeIndexSignature(h, "location", loc.Id, loc.Label)
		for _, node := range loc.NodeList {
			writeNodeSignature(h, node)
		}
		for _, resolver := range loc.DnsResolverList {
			writeResolverSignature(h, "dns_resolver", resolver)
		}
	}

	if st.Services != nil {
		for _, ws := range st.Services.Web {
			writeWebServiceSignature(h, "web_service", ws)
		}
		for _, ns := range st.Services.NameServer {
			writeResolverSignature(h, "nameserver", ns)
		}
	}

	return strconv.FormatUint(h.Sum64(), 16)
}

func writeNoticeSignature(h hash.Hash, notice Notice) {
	writeIndexSignature(h, "notice", notice.Any, notice.UpdatedAt.UnixNano(), string(notice.Html))
}

func writeVpsAdminSignature(h hash.Hash, vpsAdmin VpsAdmin) {
	writeWebServiceSignature(h, "vpsadmin_api", vpsAdmin.Api)
	writeWebServiceSignature(h, "vpsadmin_webui", vpsAdmin.Webui)
	writeWebServiceSignature(h, "vpsadmin_console", vpsAdmin.Console)
}

func writeVpsAdminLocationsSignature(h hash.Hash, locations map[int64]VpsAdminLocation) {
	ids := make([]int64, 0, len(locations))
	for id := range locations {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		return ids[i] < ids[j]
	})

	for _, id := range ids {
		loc := locations[id]
		writeIndexSignature(
			h,
			"vpsadmin_location",
			id,
			loc.Label,
			loc.EnvironmentId,
			loc.EnvironmentLabel,
		)
	}
}

func writeOutageReportsSignature(h hash.Hash, reports *OutageReports) {
	if reports == nil {
		writeIndexSignature(h, "outage_reports", "nil")
		return
	}

	writeIndexSignature(
		h,
		"outage_reports",
		reports.Status,
		reports.AnyActive,
		reports.AnyActivePlanned,
		reports.AnyActiveUnplanned,
		reports.AnyRecent,
		reports.AnyRecentPlanned,
		reports.AnyRecentUnplanned,
	)
	for _, report := range reports.ActiveList {
		writeOutageReportSignature(h, "active_outage", report)
	}
	for _, report := range reports.RecentList {
		writeOutageReportSignature(h, "recent_outage", report)
	}
}

func writeOutageReportSignature(h hash.Hash, prefix string, report *OutageReport) {
	if report == nil {
		writeIndexSignature(h, prefix, "nil")
		return
	}

	writeIndexSignature(
		h,
		prefix,
		report.Id,
		report.BeginsAt.UnixNano(),
		report.FinishedAt.UnixNano(),
		report.Duration,
		report.Type,
		report.State,
		report.Impact,
		report.CsSummary,
		report.EnSummary,
	)
	for _, entity := range report.AffectedEntities {
		writeIndexSignature(
			h,
			prefix+"_entity",
			entity.Name,
			entity.EffectiveType(),
			entity.Id,
			entity.DisplayLabel(),
		)
	}
}

func writeSecurityAdvisoriesSignature(h hash.Hash, advisories *SecurityAdvisories) {
	if advisories == nil {
		writeIndexSignature(h, "security_advisories", "nil")
		return
	}

	writeIndexSignature(
		h,
		"security_advisories",
		advisories.Status,
		advisories.AnyRecent,
	)
	for _, advisory := range advisories.RecentList {
		writeSecurityAdvisorySignature(h, "recent_security_advisory", advisory)
	}
}

func writeSecurityAdvisorySignature(h hash.Hash, prefix string, advisory *SecurityAdvisory) {
	if advisory == nil {
		writeIndexSignature(h, prefix, "nil")
		return
	}

	writeIndexSignature(
		h,
		prefix,
		advisory.Id,
		advisory.PublishedAt.UnixNano(),
		advisory.UpdatedAt.UnixNano(),
		advisory.State,
		advisory.Name,
		advisory.CsSummary,
		advisory.EnSummary,
		advisory.AffectedNodeCount,
	)
	for _, cve := range advisory.Cves {
		writeIndexSignature(h, prefix+"_cve", cve.Id, cve.CveId, cve.Url)
	}
}

func writeNodeSignature(h hash.Hash, node *Node) {
	if node == nil {
		writeIndexSignature(h, "node", "nil")
		return
	}

	writeIndexSignature(
		h,
		"node",
		node.Id,
		node.Name,
		node.LocationId,
		node.OsType,
		node.ApiStatus,
		node.ApiMaintenance,
		node.PoolState,
		node.PoolScan,
		node.PoolScanPercent,
		node.PoolStatus,
	)
	writePingSignature(h, "node_ping", node.Ping)
}

func writeResolverSignature(h hash.Hash, prefix string, resolver *DnsResolver) {
	if resolver == nil {
		writeIndexSignature(h, prefix, "nil")
		return
	}

	writeIndexSignature(
		h,
		prefix,
		resolver.Name,
		resolver.IpAddress,
		resolver.ResolveDomain,
		resolver.ResolveStatus,
	)
	writePingSignature(h, prefix+"_ping", resolver.Ping)
}

func writeWebServiceSignature(h hash.Hash, prefix string, ws *WebService) {
	if ws == nil {
		writeIndexSignature(h, prefix, "nil")
		return
	}

	writeIndexSignature(
		h,
		prefix,
		ws.Label,
		ws.Description,
		ws.Url,
		ws.CheckUrl,
		ws.Method,
		ws.Status,
		ws.Maintenance,
		ws.StatusCode,
	)
}

func writePingSignature(h hash.Hash, prefix string, ping *PingCheck) {
	if ping == nil {
		writeIndexSignature(h, prefix, "nil")
		return
	}

	writeIndexSignature(h, prefix, ping.Name, ping.IpAddress, ping.PacketLoss)
}

func writeIndexSignature(h hash.Hash, values ...any) {
	for _, value := range values {
		fmt.Fprintf(h, "%v", value)
		h.Write([]byte{0})
	}
	h.Write([]byte{'\n'})
}
