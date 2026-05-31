package main

import (
	"bytes"
	"compress/gzip"
	"errors"
	"html/template"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	dynamicCacheTTL = time.Second
	noticeCacheTTL  = time.Second
)

type cachedResponse struct {
	statusCode   int
	contentType  string
	cacheControl string
	body         []byte
	gzipBody     []byte
	expiresAt    time.Time
}

type responseCache struct {
	mu      sync.Mutex
	entries map[string]cachedResponse
}

type responsePayload struct {
	statusCode   int
	contentType  string
	cacheControl string
	body         []byte
	cacheFor     time.Duration
}

func newResponseCache() *responseCache {
	return &responseCache{
		entries: make(map[string]cachedResponse),
	}
}

func (c *responseCache) get(key string, now time.Time) (cachedResponse, bool) {
	if c == nil {
		return cachedResponse{}, false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[key]
	if !ok {
		return cachedResponse{}, false
	}
	if !now.Before(entry.expiresAt) {
		delete(c.entries, key)
		return cachedResponse{}, false
	}

	return entry, true
}

func (c *responseCache) set(key string, payload responsePayload, now time.Time) cachedResponse {
	entry := newCachedResponse(payload, now)

	if c == nil || payload.cacheFor <= 0 || payload.statusCode != http.StatusOK {
		return entry
	}

	c.mu.Lock()
	c.pruneExpiredLocked(now)
	c.entries[key] = entry
	c.mu.Unlock()

	return entry
}

func newCachedResponse(payload responsePayload, now time.Time) cachedResponse {
	entry := cachedResponse{
		statusCode:   payload.statusCode,
		contentType:  payload.contentType,
		cacheControl: payload.cacheControl,
		body:         append([]byte(nil), payload.body...),
		expiresAt:    now.Add(payload.cacheFor),
	}
	if len(entry.body) > 0 {
		entry.gzipBody = gzipBytes(entry.body)
	}

	return entry
}

func (c *responseCache) pruneExpiredLocked(now time.Time) {
	for key, entry := range c.entries {
		if !now.Before(entry.expiresAt) {
			delete(c.entries, key)
		}
	}
}

func routeCacheKey(r *http.Request) string {
	if r == nil || r.URL == nil {
		return ""
	}

	query := r.URL.Query().Encode()
	if query == "" {
		return r.URL.Path
	}

	return r.URL.Path + "?" + query
}

func (app *application) serveCachedResponse(w http.ResponseWriter, r *http.Request, key string, build func(time.Time) (responsePayload, error)) {
	app.ensureCaches()
	now := app.currentTime()

	if entry, ok := app.responseCache.get(key, now); ok {
		writeResponse(w, r, &entry)
		return
	}

	payload, err := build(now)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if payload.statusCode == 0 {
		payload.statusCode = http.StatusOK
	}

	entry := app.responseCache.set(key, payload, now)
	writeResponse(w, r, &entry)
}

func writeResponse(w http.ResponseWriter, r *http.Request, entry *cachedResponse) {
	if entry == nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if entry.contentType != "" {
		w.Header().Set("Content-Type", entry.contentType)
	}
	if entry.cacheControl != "" {
		w.Header().Set("Cache-Control", entry.cacheControl)
	}
	w.Header().Add("Vary", "Accept-Encoding")

	body := entry.body
	if requestAcceptsGzip(r) {
		if len(entry.gzipBody) > 0 {
			w.Header().Set("Content-Encoding", "gzip")
			body = entry.gzipBody
		} else if len(entry.body) > 0 {
			if gzipBody := gzipBytes(entry.body); len(gzipBody) > 0 {
				w.Header().Set("Content-Encoding", "gzip")
				body = gzipBody
			}
		}
	}

	w.WriteHeader(entry.statusCode)
	if r == nil || r.Method != http.MethodHead {
		_, _ = w.Write(body)
	}
}

func gzipBytes(body []byte) []byte {
	var buf bytes.Buffer
	if !writeGzipChunks(&buf, body) {
		return nil
	}
	return buf.Bytes()
}

func writeGzipChunks(w io.Writer, chunks ...[]byte) bool {
	zw := gzipWriterPool.Get().(*gzip.Writer)
	defer gzipWriterPool.Put(zw)

	zw.Reset(w)
	for _, chunk := range chunks {
		if len(chunk) == 0 {
			continue
		}
		if _, err := zw.Write(chunk); err != nil {
			return false
		}
	}
	if err := zw.Close(); err != nil {
		return false
	}
	return true
}

var gzipWriterPool = sync.Pool{
	New: func() any {
		zw, err := gzip.NewWriterLevel(io.Discard, gzip.BestSpeed)
		if err != nil {
			return gzip.NewWriter(io.Discard)
		}
		return zw
	},
}

func requestAcceptsGzip(r *http.Request) bool {
	if r == nil {
		return false
	}

	for _, part := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		encoding := strings.ToLower(strings.TrimSpace(strings.SplitN(part, ";", 2)[0]))
		if encoding == "gzip" {
			return true
		}
	}

	return false
}

type noticeCache struct {
	mu        sync.Mutex
	path      string
	notice    Notice
	exists    bool
	size      int64
	modTime   time.Time
	checkedAt time.Time
}

func newNoticeCache() *noticeCache {
	return &noticeCache{}
}

func (c *noticeCache) read(path string, now time.Time) (Notice, error) {
	if c == nil {
		return readNoticeFile(path)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.path == path && !c.checkedAt.IsZero() && now.Before(c.checkedAt.Add(noticeCacheTTL)) {
		return c.notice, nil
	}

	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.path = path
			c.notice = Notice{}
			c.exists = false
			c.size = 0
			c.modTime = time.Time{}
			c.checkedAt = now
			return c.notice, nil
		}
		return Notice{}, err
	}

	if c.path == path && c.exists && c.size == info.Size() && c.modTime.Equal(info.ModTime()) {
		c.checkedAt = now
		return c.notice, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.path = path
			c.notice = Notice{}
			c.exists = false
			c.size = 0
			c.modTime = time.Time{}
			c.checkedAt = now
			return c.notice, nil
		}
		return Notice{}, err
	}

	c.path = path
	c.notice = Notice{
		Any:       true,
		Html:      template.HTML(data),
		UpdatedAt: info.ModTime(),
	}
	c.exists = true
	c.size = info.Size()
	c.modTime = info.ModTime()
	c.checkedAt = now

	return c.notice, nil
}
