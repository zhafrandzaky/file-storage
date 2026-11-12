// Package httpapi exposes the JSON API and serves the web UI.
package httpapi

import (
	"context"
	"io"
	"io/fs"
	"net/http"
	"time"

	"github.com/zhafrandzaky/file-storage/internal/audit"
	"github.com/zhafrandzaky/file-storage/internal/auth"
	"github.com/zhafrandzaky/file-storage/internal/files"
	"github.com/zhafrandzaky/file-storage/internal/storage"
	"github.com/zhafrandzaky/file-storage/internal/users"
)

// ObjectStore is the object storage behaviour the HTTP layer depends on.
type ObjectStore interface {
	Bucket() string
	Ping(ctx context.Context) error
	Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error
	Get(ctx context.Context, key string) (*storage.Download, error)
	Delete(ctx context.Context, key string) error
	ShareEnabled() bool
	PresignDownload(ctx context.Context, key, filename string, ttl time.Duration) (string, error)
}

// Deps are the collaborators and settings of the HTTP layer.
type Deps struct {
	Objects        ObjectStore
	Files          *files.Store
	Users          *users.Store
	Sessions       *auth.Sessions
	Audit          *audit.Log
	Policy         files.Policy
	MaxUploadBytes int64
	ShareMaxTTL    time.Duration
	CookieSecure   bool
	Endpoint       string // internal S3 endpoint, shown to admins
	Static         fs.FS  // built web UI; nil serves the API only
}

type api struct {
	Deps
	throttle *auth.Throttle
}

// New returns the root handler.
func New(d Deps) http.Handler {
	a := &api{Deps: d, throttle: auth.NewThrottle(5, 15*time.Minute)}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", a.health)

	mux.HandleFunc("POST /api/auth/login", a.login)
	mux.Handle("POST /api/auth/logout", a.authed(a.logout))
	mux.Handle("GET /api/auth/me", a.authed(a.me))
	mux.Handle("POST /api/auth/password", a.authed(a.changePassword))

	mux.Handle("GET /api/info", a.authed(a.info))
	mux.Handle("GET /api/files", a.authed(a.listFiles))
	mux.Handle("POST /api/files", a.authed(a.uploadFiles))
	mux.Handle("GET /api/files/{id}", a.authed(a.downloadFile))
	mux.Handle("DELETE /api/files/{id}", a.authed(a.deleteFile))
	mux.Handle("POST /api/files/{id}/share", a.authed(a.shareFile))

	mux.Handle("GET /api/users", a.admin(a.listUsers))
	mux.Handle("POST /api/users", a.admin(a.createUser))
	mux.Handle("PATCH /api/users/{id}", a.admin(a.updateUser))
	mux.Handle("POST /api/users/{id}/password", a.admin(a.resetPassword))
	mux.Handle("GET /api/audit", a.admin(a.listAudit))

	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "Endpoint tidak ditemukan.")
	})
	if d.Static != nil {
		mux.Handle("/", spa(d.Static))
	}
	return recoverer(logger(securityHeaders(csrf(mux))))
}

func (a *api) health(w http.ResponseWriter, r *http.Request) {
	if err := a.Objects.Ping(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "degraded", "storage": "unreachable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "storage": "ok"})
}
