package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

// ErrNoSession is returned for missing, expired or revoked sessions.
var ErrNoSession = errors.New("no valid session")

// Sessions stores opaque session tokens. Only a SHA-256 of each token is kept,
// so a leaked database does not leak usable cookies.
type Sessions struct {
	db  *sql.DB
	ttl time.Duration
}

// NewSessions returns a session store with the given lifetime.
func NewSessions(db *sql.DB, ttl time.Duration) *Sessions { return &Sessions{db: db, ttl: ttl} }

// TTL returns the session lifetime.
func (s *Sessions) TTL() time.Duration { return s.ttl }

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// Create starts a session and returns the raw token for the cookie.
func (s *Sessions) Create(ctx context.Context, userID int64) (string, time.Time, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", time.Time{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	now := time.Now()
	expires := now.Add(s.ttl)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions (token_hash, user_id, expires_at, created_at) VALUES (?, ?, ?, ?)`,
		hashToken(token), userID, expires.Unix(), now.Unix())
	return token, expires, err
}

// UserID returns the user owning a valid session token.
func (s *Sessions) UserID(ctx context.Context, token string) (int64, error) {
	if token == "" {
		return 0, ErrNoSession
	}
	var id int64
	err := s.db.QueryRowContext(ctx,
		`SELECT s.user_id FROM sessions s JOIN users u ON u.id = s.user_id
		 WHERE s.token_hash = ? AND s.expires_at > ? AND u.active = 1`,
		hashToken(token), time.Now().Unix()).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNoSession
	}
	return id, err
}

// Delete ends one session.
func (s *Sessions) Delete(ctx context.Context, token string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, hashToken(token))
	return err
}

// DeleteExpired removes stale sessions.
func (s *Sessions) DeleteExpired(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, time.Now().Unix())
	return err
}

// Throttle limits failed login attempts per key within a sliding window.
type Throttle struct {
	mu       sync.Mutex
	limit    int
	window   time.Duration
	failures map[string][]time.Time
}

// NewThrottle allows limit failures per window for each key.
func NewThrottle(limit int, window time.Duration) *Throttle {
	return &Throttle{limit: limit, window: window, failures: map[string][]time.Time{}}
}

func (t *Throttle) prune(key string, now time.Time) []time.Time {
	kept := t.failures[key][:0]
	for _, at := range t.failures[key] {
		if now.Sub(at) < t.window {
			kept = append(kept, at)
		}
	}
	if len(kept) == 0 {
		delete(t.failures, key)
		return nil
	}
	t.failures[key] = kept
	return kept
}

// Blocked reports whether key has exceeded the limit.
func (t *Throttle) Blocked(key string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.prune(key, time.Now())) >= t.limit
}

// Fail records a failed attempt.
func (t *Throttle) Fail(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	t.failures[key] = append(t.prune(key, now), now)
}

// Reset clears failures after a successful login.
func (t *Throttle) Reset(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.failures, key)
}
