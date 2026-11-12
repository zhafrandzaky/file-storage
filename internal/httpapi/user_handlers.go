package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/zhafrandzaky/file-storage/internal/audit"
	"github.com/zhafrandzaky/file-storage/internal/auth"
	"github.com/zhafrandzaky/file-storage/internal/users"
)

func userError(w http.ResponseWriter, op string, err error) {
	switch {
	case errors.Is(err, users.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Pengguna tidak ditemukan.")
	case errors.Is(err, users.ErrEmailTaken):
		writeError(w, http.StatusConflict, "email_taken", "Email tersebut sudah terdaftar.")
	case errors.Is(err, users.ErrInvalidEmail):
		writeError(w, http.StatusBadRequest, "invalid_email", "Format email tidak valid.")
	case errors.Is(err, users.ErrInvalidName):
		writeError(w, http.StatusBadRequest, "invalid_name", "Nama wajib diisi, maksimal 100 karakter.")
	case errors.Is(err, users.ErrInvalidRole):
		writeError(w, http.StatusBadRequest, "invalid_role", "Peran harus admin atau member.")
	case errors.Is(err, users.ErrLastAdmin):
		writeError(w, http.StatusConflict, "last_admin", "Harus ada minimal satu administrator aktif.")
	case errors.Is(err, auth.ErrWeakPassword):
		writeError(w, http.StatusBadRequest, "weak_password", "Kata sandi minimal 8 karakter.")
	default:
		internalError(w, op, err)
	}
}

func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		writeError(w, http.StatusNotFound, "not_found", "Pengguna tidak ditemukan.")
		return 0, false
	}
	return id, true
}

func (a *api) listUsers(w http.ResponseWriter, r *http.Request) {
	list, err := a.Users.List(r.Context())
	if err != nil {
		internalError(w, "list users", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": list})
}

func (a *api) createUser(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	u, err := a.Users.Create(r.Context(), users.NewUser{Name: body.Name, Email: body.Email, Password: body.Password, Role: body.Role})
	if err != nil {
		userError(w, "create user", err)
		return
	}
	a.record(r, audit.UserCreate, u.Email, "peran "+u.Role)
	writeJSON(w, http.StatusCreated, u)
}

func (a *api) updateUser(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body struct {
		Role   *string `json:"role"`
		Active *bool   `json:"active"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if id == currentUser(r).ID {
		writeError(w, http.StatusBadRequest, "self_update", "Anda tidak dapat mengubah peran atau status akun sendiri.")
		return
	}
	u, err := a.Users.Update(r.Context(), id, body.Role, body.Active)
	if err != nil {
		userError(w, "update user", err)
		return
	}
	a.record(r, audit.UserUpdate, u.Email, fmt.Sprintf("peran %s, aktif %t", u.Role, u.Active))
	writeJSON(w, http.StatusOK, u)
}

func (a *api) resetPassword(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body struct {
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	u, err := a.Users.ByID(r.Context(), id)
	if err != nil {
		userError(w, "get user", err)
		return
	}
	if err := a.Users.SetPassword(r.Context(), id, body.Password); err != nil {
		userError(w, "reset password", err)
		return
	}
	a.record(r, audit.UserPassword, u.Email, "")
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) listAudit(w http.ResponseWriter, r *http.Request) {
	limit := 200
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 && v <= 1000 {
		limit = v
	}
	entries, err := a.Audit.List(r.Context(), limit)
	if err != nil {
		internalError(w, "list audit", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries})
}
