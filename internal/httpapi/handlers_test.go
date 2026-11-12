package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/zhafrandzaky/file-storage/internal/storage"
)

type memStore struct {
	mu    sync.Mutex
	files map[string][]byte
	types map[string]string
}

func newMemStore() *memStore {
	return &memStore{files: map[string][]byte{}, types: map[string]string{}}
}

func (m *memStore) Bucket() string                 { return "test-bucket" }
func (m *memStore) Ping(ctx context.Context) error { return nil }

func (m *memStore) List(ctx context.Context) ([]storage.Object, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []storage.Object{}
	for k, v := range m.files {
		out = append(out, storage.Object{Key: k, Size: int64(len(v)), LastModified: time.Now()})
	}
	return out, nil
}

func (m *memStore) Put(ctx context.Context, key string, r io.Reader, size int64, ct string) error {
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files[key], m.types[key] = b, ct
	return nil
}

func (m *memStore) Get(ctx context.Context, key string) (*storage.Download, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.files[key]
	if !ok {
		return nil, storage.ErrNotFound
	}
	return &storage.Download{Body: io.NopCloser(bytes.NewReader(b)), ContentType: m.types[key], Size: int64(len(b))}, nil
}

func (m *memStore) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.files[key]; !ok {
		return storage.ErrNotFound
	}
	delete(m.files, key)
	return nil
}

func newServer(store Store, max int64) *httptest.Server {
	return httptest.NewServer(New(Options{Store: store, MaxUploadBytes: max, Endpoint: "http://test"}))
}

func uploadReq(t *testing.T, url string, files map[string]string) *http.Response {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for name, content := range files {
		fw, _ := mw.CreateFormFile("files", name)
		fw.Write([]byte(content))
	}
	mw.Close()
	resp, err := http.Post(url+"/api/files", mw.FormDataContentType(), &body)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestUploadListDownloadDelete(t *testing.T) {
	srv := newServer(newMemStore(), 1<<20)
	defer srv.Close()

	resp := uploadReq(t, srv.URL, map[string]string{"annual report.pdf": "report body"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("upload status = %d", resp.StatusCode)
	}

	resp, _ = http.Get(srv.URL + "/api/files")
	var list struct {
		Files     []storage.Object `json:"files"`
		TotalSize int64            `json:"totalSize"`
	}
	json.NewDecoder(resp.Body).Decode(&list)
	if len(list.Files) != 1 || list.Files[0].Key != "annual report.pdf" || list.TotalSize != 11 {
		t.Fatalf("unexpected list: %+v", list)
	}

	resp, _ = http.Get(srv.URL + "/api/files/annual%20report.pdf?download")
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || string(b) != "report body" {
		t.Fatalf("download = %d %q", resp.StatusCode, b)
	}
	if cd := resp.Header.Get("Content-Disposition"); cd != "attachment; filename*=UTF-8''annual%20report.pdf" {
		t.Errorf("Content-Disposition = %q", cd)
	}

	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/files/annual%20report.pdf", nil)
	resp, _ = http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status = %d", resp.StatusCode)
	}
	resp, _ = http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("second delete status = %d, want 404", resp.StatusCode)
	}
}

func TestUploadRejectsOversizedBody(t *testing.T) {
	srv := newServer(newMemStore(), 1024)
	defer srv.Close()
	resp := uploadReq(t, srv.URL, map[string]string{"large.bin": string(make([]byte, 4096))})
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", resp.StatusCode)
	}
}

func TestUploadRequiresFiles(t *testing.T) {
	srv := newServer(newMemStore(), 1<<20)
	defer srv.Close()
	resp := uploadReq(t, srv.URL, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestCleanKey(t *testing.T) {
	cases := map[string]string{
		"../../etc/passwd":         "passwd",
		`C:\Users\a\contract.docx`: "contract.docx",
		"  notes.txt ":             "notes.txt",
	}
	for in, want := range cases {
		if got, ok := cleanKey(in); !ok || got != want {
			t.Errorf("cleanKey(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
	for _, bad := range []string{"", "..", "/", "a\x00b"} {
		if _, ok := cleanKey(bad); ok {
			t.Errorf("cleanKey(%q) should be rejected", bad)
		}
	}
}
