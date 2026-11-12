package httpapi

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"sort"
	"time"

	"github.com/zhafrandzaky/file-storage/internal/audit"
	"github.com/zhafrandzaky/file-storage/internal/files"
)

func (a *api) info(w http.ResponseWriter, r *http.Request) {
	exts := make([]string, 0, len(a.Policy.AllowedExt))
	for ext := range a.Policy.AllowedExt {
		exts = append(exts, ext)
	}
	sort.Strings(exts)
	body := map[string]any{
		"bucket":            a.Objects.Bucket(),
		"maxUploadBytes":    a.MaxUploadBytes,
		"maxFileBytes":      a.Policy.MaxFileBytes,
		"allowedExtensions": exts,
		"shareEnabled":      a.Objects.ShareEnabled(),
		"shareMaxHours":     int(a.ShareMaxTTL.Hours()),
	}
	if currentUser(r).IsAdmin() {
		body["endpoint"] = a.Endpoint
	}
	writeJSON(w, http.StatusOK, body)
}

func (a *api) listFiles(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := files.Filter{Kind: q.Get("kind"), Query: q.Get("q")}
	if filter.Kind != "" && !files.ValidKind(filter.Kind) {
		writeError(w, http.StatusBadRequest, "invalid_kind", "Jenis berkas tidak dikenal.")
		return
	}
	list, err := a.Files.List(r.Context(), filter)
	if err != nil {
		internalError(w, "list files", err)
		return
	}
	summary, err := a.Files.Summarize(r.Context())
	if err != nil {
		internalError(w, "summarize files", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"files": list, "summary": summary})
}

type pendingUpload struct {
	part *multipart.FileHeader
	name string
	mime string
}

func uploadError(err error, name string, maxFile int64) string {
	switch {
	case errors.Is(err, files.ErrInvalidName):
		return fmt.Sprintf("Nama berkas %q tidak valid.", name)
	case errors.Is(err, files.ErrExtNotAllowed):
		return fmt.Sprintf("Jenis berkas %s tidak diizinkan.", name)
	case errors.Is(err, files.ErrTooLarge):
		return fmt.Sprintf("%s melebihi batas %d MB per berkas.", name, maxFile>>20)
	case errors.Is(err, files.ErrEmpty):
		return fmt.Sprintf("%s kosong.", name)
	case errors.Is(err, files.ErrContentInvalid):
		return fmt.Sprintf("Isi %s tidak sesuai dengan jenis berkasnya.", name)
	}
	return fmt.Sprintf("%s tidak dapat diproses.", name)
}

func (a *api) uploadFiles(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, a.MaxUploadBytes)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, "too_large",
				fmt.Sprintf("Total unggahan melebihi batas %d MB. Unggah berkas secara bertahap.", a.MaxUploadBytes>>20))
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

	// Validate everything first so a bad file does not leave a partial upload.
	pending := make([]pendingUpload, 0, len(parts))
	for _, part := range parts {
		name, err := files.CleanName(part.Filename)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_file", uploadError(err, part.Filename, a.Policy.MaxFileBytes))
			return
		}
		head, err := readHead(part)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_file", uploadError(err, name, a.Policy.MaxFileBytes))
			return
		}
		ct, err := a.Policy.Check(name, part.Size, head)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, "invalid_file", uploadError(err, name, a.Policy.MaxFileBytes))
			return
		}
		pending = append(pending, pendingUpload{part: part, name: name, mime: ct})
	}

	user := currentUser(r)
	stored := make([]files.File, 0, len(pending))
	for _, p := range pending {
		now := time.Now().UTC()
		f := files.File{
			ID: files.NewID(), Name: p.name, Size: p.part.Size, MimeType: p.mime, Kind: files.KindOf(p.name),
			OwnerID: user.ID, OwnerName: user.Name, CreatedAt: now,
		}
		f.ObjectKey = files.ObjectKey(f.ID, f.Name, now)

		src, err := p.part.Open()
		if err != nil {
			internalError(w, "open part", err)
			return
		}
		err = a.Objects.Put(r.Context(), f.ObjectKey, src, f.Size, f.MimeType)
		src.Close()
		if err != nil {
			slog.Error("put object", "key", f.ObjectKey, "err", err)
			writeError(w, http.StatusBadGateway, "storage_unavailable", "Penyimpanan tidak merespons. Periksa layanan RustFS, lalu coba lagi.")
			return
		}
		if err := a.Files.Insert(r.Context(), f); err != nil {
			_ = a.Objects.Delete(r.Context(), f.ObjectKey)
			internalError(w, "insert file", err)
			return
		}
		a.record(r, audit.FileUpload, f.Name, f.ObjectKey)
		stored = append(stored, f)
	}
	writeJSON(w, http.StatusCreated, map[string]any{"uploaded": stored})
}

func readHead(part *multipart.FileHeader) ([]byte, error) {
	f, err := part.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	head := make([]byte, 512)
	n, err := io.ReadFull(f, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return head[:n], nil
}

func (a *api) loadFile(w http.ResponseWriter, r *http.Request) (files.File, bool) {
	f, err := a.Files.Get(r.Context(), r.PathValue("id"))
	if errors.Is(err, files.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "Berkas tidak ditemukan.")
		return f, false
	}
	if err != nil {
		internalError(w, "get file", err)
		return f, false
	}
	return f, true
}

func (a *api) downloadFile(w http.ResponseWriter, r *http.Request) {
	f, ok := a.loadFile(w, r)
	if !ok {
		return
	}
	obj, err := a.Objects.Get(r.Context(), f.ObjectKey)
	if err != nil {
		slog.Error("get object", "key", f.ObjectKey, "err", err)
		writeError(w, http.StatusBadGateway, "storage_unavailable", "Berkas tidak dapat diambil dari penyimpanan.")
		return
	}
	defer obj.Body.Close()

	disposition := "inline"
	if r.URL.Query().Has("download") {
		disposition = "attachment"
	}
	w.Header().Set("Content-Type", f.MimeType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("%s; filename*=UTF-8''%s", disposition, url.PathEscape(f.Name)))
	w.Header().Set("Content-Length", fmt.Sprint(f.Size))
	if _, err := io.Copy(w, obj.Body); err != nil && !isClientGone(err) {
		slog.Warn("stream object", "key", f.ObjectKey, "err", err)
	}
}

func (a *api) deleteFile(w http.ResponseWriter, r *http.Request) {
	f, ok := a.loadFile(w, r)
	if !ok {
		return
	}
	user := currentUser(r)
	if !user.IsAdmin() && f.OwnerID != user.ID {
		writeError(w, http.StatusForbidden, "forbidden", "Anda hanya dapat menghapus berkas milik Anda sendiri.")
		return
	}
	if err := a.Objects.Delete(r.Context(), f.ObjectKey); err != nil {
		slog.Error("delete object", "key", f.ObjectKey, "err", err)
		writeError(w, http.StatusBadGateway, "storage_unavailable", "Penyimpanan tidak merespons. Berkas belum dihapus.")
		return
	}
	if err := a.Files.Delete(r.Context(), f.ID); err != nil && !errors.Is(err, files.ErrNotFound) {
		internalError(w, "delete file", err)
		return
	}
	a.record(r, audit.FileDelete, f.Name, f.ObjectKey)
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) shareFile(w http.ResponseWriter, r *http.Request) {
	if !a.Objects.ShareEnabled() {
		writeError(w, http.StatusConflict, "share_disabled", "Tautan berbagi belum diaktifkan. Atur S3_PUBLIC_ENDPOINT di konfigurasi.")
		return
	}
	var body struct {
		Hours int `json:"hours"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	ttl := time.Duration(body.Hours) * time.Hour
	if body.Hours < 1 || ttl > a.ShareMaxTTL {
		writeError(w, http.StatusBadRequest, "invalid_duration",
			fmt.Sprintf("Masa berlaku tautan harus antara 1 dan %d jam.", int(a.ShareMaxTTL.Hours())))
		return
	}
	f, ok := a.loadFile(w, r)
	if !ok {
		return
	}
	link, err := a.Objects.PresignDownload(r.Context(), f.ObjectKey, f.Name, ttl)
	if err != nil {
		internalError(w, "presign", err)
		return
	}
	expires := time.Now().Add(ttl).UTC()
	a.record(r, audit.FileShare, f.Name, fmt.Sprintf("berlaku %d jam", body.Hours))
	writeJSON(w, http.StatusOK, map[string]any{"url": link, "expiresAt": expires})
}
