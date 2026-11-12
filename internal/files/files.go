// Package files keeps file metadata and the upload policy. File bytes live in
// object storage; this package only knows their object keys.
package files

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"
)

// Kinds used for filtering.
const (
	KindDocument    = "document"
	KindImage       = "image"
	KindSpreadsheet = "spreadsheet"
	KindArchive     = "archive"
	KindOther       = "other"
)

var kindByExt = map[string]string{
	"pdf": KindDocument, "doc": KindDocument, "docx": KindDocument, "odt": KindDocument,
	"txt": KindDocument, "md": KindDocument, "rtf": KindDocument, "ppt": KindDocument,
	"pptx": KindDocument, "odp": KindDocument,
	"png": KindImage, "jpg": KindImage, "jpeg": KindImage, "gif": KindImage, "webp": KindImage, "bmp": KindImage,
	"xls": KindSpreadsheet, "xlsx": KindSpreadsheet, "csv": KindSpreadsheet, "ods": KindSpreadsheet,
	"zip": KindArchive, "7z": KindArchive, "tar": KindArchive, "gz": KindArchive, "rar": KindArchive,
}

// ValidKind reports whether k is a known kind filter.
func ValidKind(k string) bool {
	switch k {
	case KindDocument, KindImage, KindSpreadsheet, KindArchive, KindOther:
		return true
	}
	return false
}

// Ext returns the lower-case extension without the dot.
func Ext(name string) string {
	return strings.TrimPrefix(strings.ToLower(path.Ext(name)), ".")
}

// KindOf classifies a file by extension.
func KindOf(name string) string {
	if k, ok := kindByExt[Ext(name)]; ok {
		return k
	}
	return KindOther
}

// File is the metadata of one stored object.
type File struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Size      int64     `json:"size"`
	MimeType  string    `json:"mimeType"`
	Kind      string    `json:"kind"`
	OwnerID   int64     `json:"ownerId"`
	OwnerName string    `json:"ownerName"`
	ObjectKey string    `json:"-"`
	CreatedAt time.Time `json:"createdAt"`
}

// Validation errors.
var (
	ErrNotFound       = errors.New("file not found")
	ErrInvalidName    = errors.New("invalid file name")
	ErrExtNotAllowed  = errors.New("file type not allowed")
	ErrTooLarge       = errors.New("file too large")
	ErrEmpty          = errors.New("file is empty")
	ErrContentInvalid = errors.New("file content does not match its extension")
)

// Policy decides which uploads are accepted.
type Policy struct {
	AllowedExt   map[string]bool
	MaxFileBytes int64
}

// CleanName turns a client filename into a safe display name.
func CleanName(name string) (string, error) {
	name = strings.ReplaceAll(name, "\\", "/")
	name = strings.TrimSpace(path.Base(name))
	if name == "" || name == "." || name == "/" || name == ".." || len(name) > 255 {
		return "", ErrInvalidName
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return "", ErrInvalidName
		}
	}
	return name, nil
}

// Check validates name, size and the first bytes of content. It returns the
// MIME type to store, derived from the extension rather than the client.
func (p Policy) Check(name string, size int64, head []byte) (string, error) {
	ext := Ext(name)
	if ext == "" || !p.AllowedExt[ext] {
		return "", ErrExtNotAllowed
	}
	if size == 0 {
		return "", ErrEmpty
	}
	if size > p.MaxFileBytes {
		return "", ErrTooLarge
	}
	sniffed := http.DetectContentType(head)
	switch {
	case ext == "pdf" && sniffed != "application/pdf":
		return "", ErrContentInvalid
	case KindOf(name) == KindImage && !strings.HasPrefix(sniffed, "image/"):
		return "", ErrContentInvalid
	case sniffed == "application/x-msdownload" || strings.HasPrefix(string(head), "\x7fELF"):
		return "", ErrContentInvalid
	}
	ct := mime.TypeByExtension("." + ext)
	if ct == "" {
		ct = "application/octet-stream"
	}
	return ct, nil
}

// NewID returns a random RFC 4122 version 4 UUID.
func NewID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// ObjectKey builds a collision-free key: files/2026/11/<uuid>.pdf.
func ObjectKey(id, name string, at time.Time) string {
	key := fmt.Sprintf("files/%04d/%02d/%s", at.Year(), at.Month(), id)
	if ext := Ext(name); ext != "" {
		key += "." + ext
	}
	return key
}

// Store persists metadata.
type Store struct{ db *sql.DB }

// NewStore returns a Store backed by db.
func NewStore(db *sql.DB) *Store { return &Store{db: db} }

const selectFile = `SELECT f.id, f.name, f.size, f.mime_type, f.kind, f.owner_id, u.name, f.object_key, f.created_at
FROM files f JOIN users u ON u.id = f.owner_id`

func scanFile(row interface{ Scan(...any) error }) (File, error) {
	var f File
	var created int64
	err := row.Scan(&f.ID, &f.Name, &f.Size, &f.MimeType, &f.Kind, &f.OwnerID, &f.OwnerName, &f.ObjectKey, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return File{}, ErrNotFound
	}
	f.CreatedAt = time.UnixMilli(created).UTC()
	return f, err
}

// Insert records a stored object.
func (s *Store) Insert(ctx context.Context, f File) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO files (id, owner_id, name, object_key, size, mime_type, kind, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		f.ID, f.OwnerID, f.Name, f.ObjectKey, f.Size, f.MimeType, f.Kind, f.CreatedAt.UnixMilli())
	return err
}

// Get returns one file.
func (s *Store) Get(ctx context.Context, id string) (File, error) {
	return scanFile(s.db.QueryRowContext(ctx, selectFile+` WHERE f.id = ?`, id))
}

// Filter narrows List.
type Filter struct {
	Kind  string
	Query string
}

// Summary describes the whole library regardless of filter.
type Summary struct {
	Count     int64            `json:"count"`
	TotalSize int64            `json:"totalSize"`
	ByKind    map[string]int64 `json:"byKind"`
}

// List returns files matching the filter, newest first.
func (s *Store) List(ctx context.Context, f Filter) ([]File, error) {
	q := selectFile + ` WHERE 1=1`
	var args []any
	if f.Kind != "" {
		q += ` AND f.kind = ?`
		args = append(args, f.Kind)
	}
	if term := strings.TrimSpace(f.Query); term != "" {
		q += ` AND f.name LIKE ? ESCAPE '\'`
		r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
		args = append(args, "%"+r.Replace(term)+"%")
	}
	q += ` ORDER BY f.created_at DESC`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []File{}
	for rows.Next() {
		file, err := scanFile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, file)
	}
	return out, rows.Err()
}

// Summarize counts files and bytes per kind.
func (s *Store) Summarize(ctx context.Context) (Summary, error) {
	sum := Summary{ByKind: map[string]int64{}}
	rows, err := s.db.QueryContext(ctx, `SELECT kind, COUNT(*), COALESCE(SUM(size), 0) FROM files GROUP BY kind`)
	if err != nil {
		return sum, err
	}
	defer rows.Close()
	for rows.Next() {
		var kind string
		var n, size int64
		if err := rows.Scan(&kind, &n, &size); err != nil {
			return sum, err
		}
		sum.ByKind[kind] = n
		sum.Count += n
		sum.TotalSize += size
	}
	return sum, rows.Err()
}

// Delete removes metadata.
func (s *Store) Delete(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM files WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
