// Package audit records who did what, and when.
package audit

import (
	"context"
	"database/sql"
	"log/slog"
	"time"
)

// Actions recorded in the log.
const (
	LoginSuccess   = "auth.login"
	LoginFailed    = "auth.login_failed"
	Logout         = "auth.logout"
	PasswordChange = "auth.password_change"
	FileUpload     = "file.upload"
	FileDelete     = "file.delete"
	FileShare      = "file.share"
	UserCreate     = "user.create"
	UserUpdate     = "user.update"
	UserPassword   = "user.password_reset"
)

// Entry is one log line.
type Entry struct {
	ID         int64     `json:"id"`
	ActorID    *int64    `json:"actorId,omitempty"`
	ActorEmail string    `json:"actorEmail"`
	Action     string    `json:"action"`
	Target     string    `json:"target"`
	Detail     string    `json:"detail"`
	IP         string    `json:"ip"`
	CreatedAt  time.Time `json:"createdAt"`
}

// Log persists entries.
type Log struct{ db *sql.DB }

// New returns a Log backed by db.
func New(db *sql.DB) *Log { return &Log{db: db} }

// Record stores an entry. Failures are logged but never block the request.
func (l *Log) Record(ctx context.Context, e Entry) {
	_, err := l.db.ExecContext(ctx,
		`INSERT INTO audit_log (actor_id, actor_email, action, target, detail, ip, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		e.ActorID, e.ActorEmail, e.Action, e.Target, e.Detail, e.IP, time.Now().UnixMilli())
	if err != nil {
		slog.Error("audit record", "action", e.Action, "err", err)
	}
}

// List returns the newest entries first.
func (l *Log) List(ctx context.Context, limit int) ([]Entry, error) {
	rows, err := l.db.QueryContext(ctx,
		`SELECT id, actor_id, actor_email, action, target, detail, ip, created_at FROM audit_log ORDER BY created_at DESC, id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Entry{}
	for rows.Next() {
		var e Entry
		var actor sql.NullInt64
		var created int64
		if err := rows.Scan(&e.ID, &actor, &e.ActorEmail, &e.Action, &e.Target, &e.Detail, &e.IP, &created); err != nil {
			return nil, err
		}
		if actor.Valid {
			e.ActorID = &actor.Int64
		}
		e.CreatedAt = time.UnixMilli(created).UTC()
		out = append(out, e)
	}
	return out, rows.Err()
}
