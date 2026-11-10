//go:build preview

package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:8080", "preview HTTP listen address")
	dataDir := flag.String("data-dir", ".", "directory containing templates and public assets")
	scenario := flag.String("scenario", "mixed", "synthetic scenario: mixed, healthy, loading, fetch-failure")
	flag.Parse()
	if flag.NArg() != 0 {
		log.Fatal("Preview accepts flags only; no configuration file is used")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := runPreview(ctx, *listen, *dataDir, *scenario); err != nil {
		log.Fatal(err)
	}
}

func runPreview(ctx context.Context, listen, dataDir, scenario string) error {
	stateDir, err := os.MkdirTemp("", "vpsf-status-preview-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stateDir)

	app, err := newPreviewApplication(dataDir, stateDir, scenario)
	if err != nil {
		return err
	}
	defer app.status.History.Close()

	server := &http.Server{
		Addr:              listen,
		Handler:           previewHandler(app, scenario),
		ReadHeaderTimeout: 5 * time.Second,
	}
	result := make(chan error, 1)
	go func() { result <- server.ListenAndServe() }()
	log.Printf("SYNTHETIC PREVIEW (%s), frozen at %s; listening on %s; disposable state: %s", scenario, previewNow.Format(time.RFC3339), listen, stateDir)
	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			// Join the serving goroutine before closing its SQLite state.
			server.Close()
			<-result
			return err
		}
		err := <-result
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

// Add the preview label outside the production templates, including detail and
// loading pages. JSON, metrics and static files retain their normal responses.
func previewHandler(app *application, scenario string) http.Handler {
	next := app.routes()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Vpsf-Status-Preview", scenario)
		if r.URL.Path != "/" && r.URL.Path != "/entity" && r.URL.Path != "/group" && r.URL.Path != "/about" {
			next.ServeHTTP(w, r)
			return
		}

		request := r.Clone(r.Context())
		request.Header = r.Header.Clone()
		request.Header.Del("Accept-Encoding")
		response := &previewResponse{header: w.Header(), status: http.StatusOK}
		next.ServeHTTP(response, request)
		body := response.Bytes()
		if strings.HasPrefix(response.Header().Get("Content-Type"), "text/html") {
			banner := fmt.Sprintf(`<aside class="alert alert-info m-2" role="note">Preview with sample data / Ukázka s testovacími daty (%s). Not live status / Nejde o aktuální stav.</aside>`, scenario)
			body = bytes.Replace(body, []byte("<body>"), []byte("<body>"+banner), 1)
			w.Header().Del("Content-Length")
		}
		w.WriteHeader(response.status)
		w.Write(body)
	})
}

type previewResponse struct {
	bytes.Buffer
	header http.Header
	status int
}

func (w *previewResponse) Header() http.Header    { return w.header }
func (w *previewResponse) WriteHeader(status int) { w.status = status }
