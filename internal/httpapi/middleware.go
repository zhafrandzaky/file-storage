package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/zhafrandzaky/file-storage/internal/users"
)

const (
	sessionCookie = "fs_session"
	// csrfHeader must accompany every state-changing API request. Browsers do
	// not let other sites set custom headers without a CORS preflight, which
	// this server never grants.
	csrfHeader = "X-Requested-With"
)

type ctxKey struct{}

func currentUser(r *http.Request) users.User {
	u, _ := r.Context().Value(ctxKey{}).(users.User)
	return u
}

// authed requires a valid session and puts the user in the request context.
func (a *api) authed(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(sessionCookie)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "Sesi Anda telah berakhir. Silakan masuk kembali.")
			return
		}
		id, err := a.Sessions.UserID(r.Context(), c.Value)
		if err != nil {
			a.clearCookie(w)
			writeError(w, http.StatusUnauthorized, "unauthenticated", "Sesi Anda telah berakhir. Silakan masuk kembali.")
			return
		}
		u, err := a.Users.ByID(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "Sesi Anda telah berakhir. Silakan masuk kembali.")
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, u)))
	})
}

// admin requires a session belonging to an administrator.
func (a *api) admin(next http.HandlerFunc) http.Handler {
	return a.authed(func(w http.ResponseWriter, r *http.Request) {
		if !currentUser(r).IsAdmin() {
			writeError(w, http.StatusForbidden, "forbidden", "Hanya administrator yang dapat mengakses fitur ini.")
			return
		}
		next(w, r)
	})
}

func csrf(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
		default:
			if strings.HasPrefix(r.URL.Path, "/api/") && r.Header.Get(csrfHeader) == "" {
				writeError(w, http.StatusForbidden, "csrf", "Permintaan ditolak: header "+csrfHeader+" wajib disertakan.")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
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
		if r.URL.Path == "/api/health" || !strings.HasPrefix(r.URL.Path, "/api/") {
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

// Response helpers.

type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	var b errorBody
	b.Error.Code, b.Error.Message = code, msg
	writeJSON(w, status, b)
}

func internalError(w http.ResponseWriter, op string, err error) {
	slog.Error(op, "err", err)
	writeError(w, http.StatusInternalServerError, "internal", "Terjadi kesalahan di server.")
}

func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<16))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Format permintaan tidak valid.")
		return false
	}
	return true
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func isClientGone(err error) bool {
	return errors.Is(err, context.Canceled)
}
