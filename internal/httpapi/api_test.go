package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zhafrandzaky/file-storage/internal/audit"
	"github.com/zhafrandzaky/file-storage/internal/auth"
	"github.com/zhafrandzaky/file-storage/internal/database"
	"github.com/zhafrandzaky/file-storage/internal/files"
	"github.com/zhafrandzaky/file-storage/internal/storage"
	"github.com/zhafrandzaky/file-storage/internal/users"
)

// memObjects is an in-memory ObjectStore.
type memObjects struct {
	mu      sync.Mutex
	objects map[string][]byte
	share   bool
}

func (m *memObjects) Bucket() string                 { return "test-bucket" }
func (m *memObjects) Ping(ctx context.Context) error { return nil }
func (m *memObjects) ShareEnabled() bool             { return m.share }

func (m *memObjects) Put(ctx context.Context, key string, r io.Reader, size int64, ct string) error {
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objects[key] = b
	return nil
}

func (m *memObjects) Get(ctx context.Context, key string) (*storage.Download, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.objects[key]
	if !ok {
		return nil, storage.ErrNotFound
	}
	return &storage.Download{Body: io.NopCloser(bytes.NewReader(b)), Size: int64(len(b))}, nil
}

func (m *memObjects) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.objects, key)
	return nil
}

func (m *memObjects) PresignDownload(ctx context.Context, key, name string, ttl time.Duration) (string, error) {
	return fmt.Sprintf("http://public:9000/test-bucket/%s?X-Amz-Expires=%d", key, int(ttl.Seconds())), nil
}

type env struct {
	srv     *httptest.Server
	objects *memObjects
	users   *users.Store
}

const adminEmail, adminPass = "admin@example.com", "admin-password"

func setup(t *testing.T) *env {
	t.Helper()
	ctx := context.Background()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	us := users.NewStore(db)
	if _, err := us.EnsureAdmin(ctx, "Admin", adminEmail, adminPass); err != nil {
		t.Fatal(err)
	}
	objects := &memObjects{objects: map[string][]byte{}}
	srv := httptest.NewServer(New(Deps{
		Objects:        objects,
		Files:          files.NewStore(db),
		Users:          us,
		Sessions:       auth.NewSessions(db, time.Hour),
		Audit:          audit.New(db),
		Policy:         files.Policy{AllowedExt: map[string]bool{"pdf": true, "txt": true, "png": true}, MaxFileBytes: 1 << 20},
		MaxUploadBytes: 2 << 20,
		ShareMaxTTL:    168 * time.Hour,
		Endpoint:       "http://rustfs:9000",
	}))
	t.Cleanup(srv.Close)
	return &env{srv: srv, objects: objects, users: us}
}

type client struct {
	t    *testing.T
	base string
	http *http.Client
}

func (e *env) client(t *testing.T) *client {
	jar, _ := cookiejar.New(nil)
	return &client{t: t, base: e.srv.URL, http: &http.Client{Jar: jar}}
}

func (c *client) do(method, path string, body any, csrf bool) (*http.Response, []byte) {
	c.t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, c.base+path, r)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if csrf {
		req.Header.Set(csrfHeader, "test")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	return resp, out
}

func (c *client) json(method, path string, body any) (*http.Response, []byte) {
	return c.do(method, path, body, true)
}

func (c *client) login(email, password string) int {
	resp, _ := c.json(http.MethodPost, "/api/auth/login", map[string]string{"email": email, "password": password})
	return resp.StatusCode
}

func (c *client) upload(name string, content []byte) (*http.Response, []byte) {
	c.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("files", name)
	fw.Write(content)
	mw.Close()
	req, _ := http.NewRequest(http.MethodPost, c.base+"/api/files", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set(csrfHeader, "test")
	resp, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	return resp, out
}

func expect(t *testing.T, what string, got, want int, body []byte) {
	t.Helper()
	if got != want {
		t.Fatalf("%s: status %d, want %d (%s)", what, got, want, body)
	}
}

var pdf = []byte("%PDF-1.7\nhello")

func TestAuthenticationAndCSRF(t *testing.T) {
	e := setup(t)
	c := e.client(t)

	resp, body := c.do(http.MethodGet, "/api/files", nil, false)
	expect(t, "anonymous list", resp.StatusCode, 401, body)

	resp, body = c.do(http.MethodPost, "/api/auth/login", map[string]string{"email": adminEmail, "password": adminPass}, false)
	expect(t, "login without csrf header", resp.StatusCode, 403, body)

	expect(t, "wrong password", c.login(adminEmail, "nope-nope"), 401, nil)
	expect(t, "login", c.login(adminEmail, adminPass), 200, nil)

	resp, body = c.json(http.MethodGet, "/api/auth/me", nil)
	expect(t, "me", resp.StatusCode, 200, body)
	if !strings.Contains(string(body), `"role":"admin"`) {
		t.Errorf("me = %s", body)
	}

	resp, body = c.json(http.MethodPost, "/api/auth/logout", nil)
	expect(t, "logout", resp.StatusCode, 204, body)
	resp, body = c.json(http.MethodGet, "/api/auth/me", nil)
	expect(t, "me after logout", resp.StatusCode, 401, body)
}

func TestLoginThrottle(t *testing.T) {
	e := setup(t)
	c := e.client(t)
	for i := 0; i < 5; i++ {
		c.login(adminEmail, "wrong-password")
	}
	expect(t, "sixth attempt", c.login(adminEmail, adminPass), 429, nil)
}

func TestRolesAndOwnership(t *testing.T) {
	e := setup(t)
	admin := e.client(t)
	admin.login(adminEmail, adminPass)

	resp, body := admin.json(http.MethodPost, "/api/users", map[string]string{
		"name": "Member One", "email": "member@example.com", "password": "member-password", "role": "member",
	})
	expect(t, "create member", resp.StatusCode, 201, body)
	var member users.User
	json.Unmarshal(body, &member)

	resp, body = admin.json(http.MethodPost, "/api/users", map[string]string{
		"name": "Dup", "email": "MEMBER@example.com", "password": "member-password", "role": "member",
	})
	expect(t, "duplicate email", resp.StatusCode, 409, body)

	m := e.client(t)
	expect(t, "member login", m.login("member@example.com", "member-password"), 200, nil)

	resp, body = m.json(http.MethodGet, "/api/users", nil)
	expect(t, "member lists users", resp.StatusCode, 403, body)
	resp, body = m.json(http.MethodGet, "/api/audit", nil)
	expect(t, "member reads audit", resp.StatusCode, 403, body)

	// Admin uploads; member may download but not delete it.
	resp, body = admin.upload("annual report.pdf", pdf)
	expect(t, "admin upload", resp.StatusCode, 201, body)
	var up struct{ Uploaded []files.File }
	json.Unmarshal(body, &up)
	adminFile := up.Uploaded[0]
	if !strings.HasPrefix(e.objectKeys()[0], "files/") || !strings.HasSuffix(e.objectKeys()[0], ".pdf") {
		t.Errorf("object key = %v", e.objectKeys())
	}

	resp, body = m.json(http.MethodGet, "/api/files/"+adminFile.ID+"?download", nil)
	expect(t, "member downloads", resp.StatusCode, 200, body)
	if cd := resp.Header.Get("Content-Disposition"); cd != "attachment; filename*=UTF-8''annual%20report.pdf" {
		t.Errorf("Content-Disposition = %q", cd)
	}
	resp, body = m.json(http.MethodDelete, "/api/files/"+adminFile.ID, nil)
	expect(t, "member deletes admin file", resp.StatusCode, 403, body)

	resp, body = m.upload("notes.txt", []byte("member notes"))
	expect(t, "member upload", resp.StatusCode, 201, body)
	json.Unmarshal(body, &up)
	resp, body = m.json(http.MethodDelete, "/api/files/"+up.Uploaded[0].ID, nil)
	expect(t, "member deletes own file", resp.StatusCode, 204, body)

	resp, body = admin.json(http.MethodPatch, fmt.Sprintf("/api/users/%d", member.ID), map[string]any{"active": false})
	expect(t, "deactivate member", resp.StatusCode, 200, body)
	resp, body = m.json(http.MethodGet, "/api/files", nil)
	expect(t, "deactivated member session", resp.StatusCode, 401, body)
	expect(t, "deactivated member login", m.login("member@example.com", "member-password"), 401, nil)

	me, _ := e.users.Authenticate(context.Background(), adminEmail, adminPass)
	resp, body = admin.json(http.MethodPatch, fmt.Sprintf("/api/users/%d", me.ID), map[string]any{"role": "member"})
	expect(t, "admin demotes self", resp.StatusCode, 400, body)

	resp, body = admin.json(http.MethodGet, "/api/audit", nil)
	expect(t, "audit", resp.StatusCode, 200, body)
	for _, action := range []string{"auth.login", "user.create", "file.upload", "file.delete", "user.update"} {
		if !strings.Contains(string(body), action) {
			t.Errorf("audit missing %s", action)
		}
	}
}

func TestUploadValidationAndListing(t *testing.T) {
	e := setup(t)
	c := e.client(t)
	c.login(adminEmail, adminPass)

	for name, content := range map[string][]byte{
		"tool.exe":  []byte("MZ...."),
		"fake.pdf":  []byte("not a pdf at all"),
		"empty.txt": {},
		"big.pdf":   append(append([]byte{}, pdf...), make([]byte, 1<<20)...),
	} {
		resp, body := c.upload(name, content)
		if resp.StatusCode != 422 {
			t.Errorf("%s: status %d, want 422 (%s)", name, resp.StatusCode, body)
		}
	}
	if len(e.objectKeys()) != 0 {
		t.Fatalf("rejected uploads reached storage: %v", e.objectKeys())
	}

	// Same name twice must not overwrite.
	c.upload("same.pdf", pdf)
	c.upload("same.pdf", pdf)
	c.upload("notes.txt", []byte("hello"))
	if n := len(e.objectKeys()); n != 3 {
		t.Fatalf("objects = %d, want 3", n)
	}

	resp, body := c.json(http.MethodGet, "/api/files?kind=document&q=SAME", nil)
	expect(t, "filtered list", resp.StatusCode, 200, body)
	var list struct {
		Files   []files.File
		Summary files.Summary
	}
	json.Unmarshal(body, &list)
	if len(list.Files) != 2 || list.Summary.Count != 3 || list.Files[0].OwnerName != "Admin" {
		t.Errorf("unexpected list: %s", body)
	}
	resp, body = c.json(http.MethodGet, "/api/files?kind=nope", nil)
	expect(t, "bad kind", resp.StatusCode, 400, body)
}

func TestShareLinks(t *testing.T) {
	e := setup(t)
	c := e.client(t)
	c.login(adminEmail, adminPass)
	_, body := c.upload("report.pdf", pdf)
	var up struct{ Uploaded []files.File }
	json.Unmarshal(body, &up)
	path := "/api/files/" + up.Uploaded[0].ID + "/share"

	resp, body := c.json(http.MethodPost, path, map[string]int{"hours": 24})
	expect(t, "share disabled", resp.StatusCode, 409, body)

	e.objects.share = true
	resp, body = c.json(http.MethodPost, path, map[string]int{"hours": 200})
	expect(t, "share too long", resp.StatusCode, 400, body)
	resp, body = c.json(http.MethodPost, path, map[string]int{"hours": 24})
	expect(t, "share", resp.StatusCode, 200, body)
	if !strings.Contains(string(body), "X-Amz-Expires=86400") {
		t.Errorf("share body = %s", body)
	}
}

func TestChangePasswordKeepsSession(t *testing.T) {
	e := setup(t)
	c := e.client(t)
	c.login(adminEmail, adminPass)
	resp, body := c.json(http.MethodPost, "/api/auth/password", map[string]string{"currentPassword": "wrong-one", "newPassword": "new-password-1"})
	expect(t, "wrong current", resp.StatusCode, 400, body)
	resp, body = c.json(http.MethodPost, "/api/auth/password", map[string]string{"currentPassword": adminPass, "newPassword": "new-password-1"})
	expect(t, "change", resp.StatusCode, 204, body)
	resp, body = c.json(http.MethodGet, "/api/auth/me", nil)
	expect(t, "still signed in", resp.StatusCode, 200, body)

	other := e.client(t)
	expect(t, "old password", other.login(adminEmail, adminPass), 401, nil)
	expect(t, "new password", other.login(adminEmail, "new-password-1"), 200, nil)
}

func (e *env) objectKeys() []string {
	e.objects.mu.Lock()
	defer e.objects.mu.Unlock()
	keys := []string{}
	for k := range e.objects.objects {
		keys = append(keys, k)
	}
	return keys
}
