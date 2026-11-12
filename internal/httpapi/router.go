// Package httpapi exposes the JSON API and serves the web UI.
package httpapi

import (
	"context"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/zhafrandzaky/file-storage/internal/storage"
)

// Store is the storage behaviour the HTTP layer depends on.
type Store interface {
	Bucket() string
	Ping(ctx context.Context) error
	List(ctx context.Context) ([]storage.Object, error)
	Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error
	Get(ctx context.Context, key string) (*storage.Download, error)
	Delete(ctx context.Context, key string) error
}

// Options configures the HTTP layer.
type Options struct {
	Store          Store
	MaxUploadBytes int64
	Endpoint       string // shown in the UI header
	Static         fs.FS  // built web UI; nil disables UI serving
}

// New returns the root handler.
func New(o Options) http.Handler {
	h := &handlers{store: o.Store, maxUpload: o.MaxUploadBytes, endpoint: o.Endpoint}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", h.health)
	mux.HandleFunc("GET /api/info", h.info)
	mux.HandleFunc("GET /api/files", h.list)
	mux.HandleFunc("POST /api/files", h.upload)
	mux.HandleFunc("GET /api/files/{key...}", h.download)
	mux.HandleFunc("DELETE /api/files/{key...}", h.remove)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "Endpoint tidak ditemukan.")
	})
	if o.Static != nil {
		mux.Handle("/", spa(o.Static))
	}
	return recoverer(logger(mux))
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		if r.URL.Path == "/api/health" {
			return
		}
		slog.Info("request", "method", r.Method, "path", r.URL.Path, "status", rec.status, "duration", time.Since(start).Round(time.Millisecond))
	})
}

func recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				slog.Error("panic", "value", v, "path", r.URL.Path)
				writeError(w, http.StatusInternalServerError, "internal", "Terjadi kesalahan di server.")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
