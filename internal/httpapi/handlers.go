package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/zhafrandzaky/file-storage/internal/storage"
)

type handlers struct {
	store     Store
	maxUpload int64
	endpoint  string
}

type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	var b errorBody
	b.Error.Code, b.Error.Message = code, msg
	writeJSON(w, status, b)
}

func (h *handlers) health(w http.ResponseWriter, r *http.Request) {
	if err := h.store.Ping(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "degraded", "storage": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "storage": "ok"})
}

func (h *handlers) info(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"bucket":         h.store.Bucket(),
		"endpoint":       h.endpoint,
		"maxUploadBytes": h.maxUpload,
	})
}

func (h *handlers) list(w http.ResponseWriter, r *http.Request) {
	files, err := h.store.List(r.Context())
	if err != nil {
		slog.Error("list objects", "err", err)
		writeError(w, http.StatusBadGateway, "storage_unavailable", "Penyimpanan tidak merespons. Periksa container RustFS, lalu muat ulang halaman.")
		return
	}
	var total int64
	for _, f := range files {
		total += f.Size
	}
	writeJSON(w, http.StatusOK, map[string]any{"files": files, "totalSize": total})
}

func (h *handlers) upload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, h.maxUpload)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, "too_large",
				fmt.Sprintf("Ukuran unggahan melebihi %d MB. Unggah berkas dalam beberapa bagian.", h.maxUpload>>20))
			return
		}
		writeError(w, http.StatusBadRequest, "bad_request", "Format unggahan tidak valid.")
		return
	}
	defer r.MultipartForm.RemoveAll()

	parts := r.MultipartForm.File["files"]
	if len(parts) == 0 {
		writeError(w, http.StatusBadRequest, "no_files", "Pilih minimal satu berkas sebelum mengunggah.")
		return
	}

	uploaded := make([]string, 0, len(parts))
	for _, part := range parts {
		key, ok := cleanKey(part.Filename)
		if !ok {
			writeError(w, http.StatusBadRequest, "invalid_name", fmt.Sprintf("Nama berkas %q tidak valid.", part.Filename))
			return
		}
		f, err := part.Open()
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", "Berkas tidak bisa dibaca.")
			return
		}
		ct := part.Header.Get("Content-Type")
		if ct == "" {
			ct = "application/octet-stream"
		}
		err = h.store.Put(r.Context(), key, f, part.Size, ct)
		f.Close()
		if err != nil {
			slog.Error("put object", "key", key, "err", err)
			writeError(w, http.StatusBadGateway, "storage_unavailable", "Penyimpanan tidak merespons. Periksa container RustFS, lalu coba lagi.")
			return
		}
		uploaded = append(uploaded, key)
	}
	writeJSON(w, http.StatusCreated, map[string]any{"uploaded": uploaded})
}

func (h *handlers) download(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	obj, err := h.store.Get(r.Context(), key)
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "Dokumen tidak ditemukan.")
		return
	}
	if err != nil {
		slog.Error("get object", "key", key, "err", err)
		writeError(w, http.StatusBadGateway, "storage_unavailable", "Penyimpanan tidak merespons.")
		return
	}
	defer obj.Body.Close()

	disposition := "inline"
	if r.URL.Query().Has("download") {
		disposition = "attachment"
	}
	ct := obj.ContentType
	if ct == "" {
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Disposition", fmt.Sprintf("%s; filename*=UTF-8''%s", disposition, url.PathEscape(path.Base(key))))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if obj.Size > 0 {
		w.Header().Set("Content-Length", fmt.Sprint(obj.Size))
	}
	_, _ = io.Copy(w, obj.Body)
}

func (h *handlers) remove(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	err := h.store.Delete(r.Context(), key)
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "Dokumen tidak ditemukan.")
		return
	}
	if err != nil {
		slog.Error("delete object", "key", key, "err", err)
		writeError(w, http.StatusBadGateway, "storage_unavailable", "Penyimpanan tidak merespons.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// cleanKey turns a client-supplied filename into a safe flat object key.
func cleanKey(name string) (string, bool) {
	name = strings.ReplaceAll(name, "\\", "/")
	key := strings.TrimSpace(path.Base(name))
	if key == "" || key == "." || key == "/" || key == ".." || len(key) > 255 {
		return "", false
	}
	for _, r := range key {
		if r < 0x20 || r == 0x7f {
			return "", false
		}
	}
	return key, true
}
