package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/zhafrandzaky/file-storage/internal/audit"
	"github.com/zhafrandzaky/file-storage/internal/auth"
	"github.com/zhafrandzaky/file-storage/internal/users"
)

func (a *api) setSessionCookie(w http.ResponseWriter, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		MaxAge:   int(time.Until(expires).Seconds()),
		HttpOnly: true,
		Secure:   a.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (a *api) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: a.CookieSecure, SameSite: http.SameSiteLaxMode})
}

func (a *api) record(r *http.Request, action, target, detail string) {
	u := currentUser(r)
	e := audit.Entry{ActorEmail: u.Email, Action: action, Target: target, Detail: detail, IP: clientIP(r)}
	if u.ID != 0 {
		e.ActorID = &u.ID
	}
	a.Audit.Record(r.Context(), e)
}

func (a *api) login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	email := strings.ToLower(strings.TrimSpace(body.Email))
	key := clientIP(r) + "|" + email
	if a.throttle.Blocked(key) {
		writeError(w, http.StatusTooManyRequests, "throttled", "Terlalu banyak percobaan masuk. Coba lagi dalam 15 menit.")
		return
	}
	u, err := a.Users.Authenticate(r.Context(), email, body.Password)
	if errors.Is(err, users.ErrNotFound) {
		a.throttle.Fail(key)
		a.Audit.Record(r.Context(), audit.Entry{ActorEmail: email, Action: audit.LoginFailed, IP: clientIP(r)})
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "Email atau kata sandi salah, atau akun sedang nonaktif.")
		return
	}
	if err != nil {
		internalError(w, "authenticate", err)
		return
	}
	a.throttle.Reset(key)
	token, expires, err := a.Sessions.Create(r.Context(), u.ID)
	if err != nil {
		internalError(w, "create session", err)
		return
	}
	a.setSessionCookie(w, token, expires)
	a.Audit.Record(r.Context(), audit.Entry{ActorID: &u.ID, ActorEmail: u.Email, Action: audit.LoginSuccess, IP: clientIP(r)})
	writeJSON(w, http.StatusOK, u)
}

func (a *api) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		_ = a.Sessions.Delete(r.Context(), c.Value)
	}
	a.clearCookie(w)
	a.record(r, audit.Logout, "", "")
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) me(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, currentUser(r))
}

func (a *api) changePassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	u := currentUser(r)
	if !a.Users.CheckPassword(r.Context(), u.ID, body.CurrentPassword) {
		writeError(w, http.StatusBadRequest, "wrong_password", "Kata sandi saat ini tidak sesuai.")
		return
	}
	err := a.Users.SetPassword(r.Context(), u.ID, body.NewPassword)
	if errors.Is(err, auth.ErrWeakPassword) {
		writeError(w, http.StatusBadRequest, "weak_password", "Kata sandi baru minimal 8 karakter.")
		return
	}
	if err != nil {
		internalError(w, "set password", err)
		return
	}
	// SetPassword ends every session; keep this browser signed in.
	token, expires, err := a.Sessions.Create(r.Context(), u.ID)
	if err != nil {
		internalError(w, "create session", err)
		return
	}
	a.setSessionCookie(w, token, expires)
	a.record(r, audit.PasswordChange, u.Email, "")
	w.WriteHeader(http.StatusNoContent)
}
