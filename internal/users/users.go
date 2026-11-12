// Package users stores accounts and roles.
package users

import (
	"context"
	"database/sql"
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/mattn/go-sqlite3"

	"github.com/zhafrandzaky/file-storage/internal/auth"
)

// Roles.
const (
	RoleAdmin  = "admin"
	RoleMember = "member"
)

var (
	ErrNotFound     = errors.New("user not found")
	ErrEmailTaken   = errors.New("email already registered")
	ErrInvalidEmail = errors.New("invalid email address")
	ErrInvalidRole  = errors.New("invalid role")
	ErrInvalidName  = errors.New("name is required")
	ErrLastAdmin    = errors.New("at least one active admin is required")
)

// User is an account without its password hash.
type User struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"createdAt"`
}

// IsAdmin reports whether the user has the admin role.
func (u User) IsAdmin() bool { return u.Role == RoleAdmin }

// Store persists users in SQL.
type Store struct{ db *sql.DB }

// NewStore returns a Store backed by db.
func NewStore(db *sql.DB) *Store { return &Store{db: db} }

const columns = `id, name, email, role, active, created_at`

func scan(row interface{ Scan(...any) error }) (User, error) {
	var u User
	var created int64
	err := row.Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.Active, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	u.CreatedAt = time.Unix(created, 0).UTC()
	return u, err
}

// NewUser holds the fields required to create an account.
type NewUser struct {
	Name     string
	Email    string
	Password string
	Role     string
}

func (n *NewUser) normalize() error {
	n.Name = strings.TrimSpace(n.Name)
	n.Email = strings.ToLower(strings.TrimSpace(n.Email))
	if n.Name == "" || len(n.Name) > 100 {
		return ErrInvalidName
	}
	if addr, err := mail.ParseAddress(n.Email); err != nil || addr.Address != n.Email {
		return ErrInvalidEmail
	}
	if n.Role != RoleAdmin && n.Role != RoleMember {
		return ErrInvalidRole
	}
	return nil
}

// Create adds an account.
func (s *Store) Create(ctx context.Context, n NewUser) (User, error) {
	if err := n.normalize(); err != nil {
		return User{}, err
	}
	hash, err := auth.HashPassword(n.Password)
	if err != nil {
		return User{}, err
	}
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO users (name, email, password_hash, role, active, created_at) VALUES (?, ?, ?, ?, 1, ?)`,
		n.Name, n.Email, hash, n.Role, time.Now().Unix())
	var sqliteErr sqlite3.Error
	if errors.As(err, &sqliteErr) && sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique {
		return User{}, ErrEmailTaken
	}
	if err != nil {
		return User{}, err
	}
	id, _ := res.LastInsertId()
	return s.ByID(ctx, id)
}

// ByID returns one user.
func (s *Store) ByID(ctx context.Context, id int64) (User, error) {
	return scan(s.db.QueryRowContext(ctx, `SELECT `+columns+` FROM users WHERE id = ?`, id))
}

// Authenticate returns the active user matching email and password.
func (s *Store) Authenticate(ctx context.Context, email, password string) (User, error) {
	var hash string
	row := s.db.QueryRowContext(ctx, `SELECT `+columns+`, password_hash FROM users WHERE email = ?`, strings.ToLower(strings.TrimSpace(email)))
	var u User
	var created int64
	err := row.Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.Active, &created, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		// Spend comparable time so response timing does not reveal valid emails.
		auth.VerifyPassword(dummyHash, password)
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}
	if !auth.VerifyPassword(hash, password) || !u.Active {
		return User{}, ErrNotFound
	}
	u.CreatedAt = time.Unix(created, 0).UTC()
	return u, nil
}

var dummyHash, _ = auth.HashPassword("timing-equaliser")

// CheckPassword verifies a user's current password.
func (s *Store) CheckPassword(ctx context.Context, id int64, password string) bool {
	var hash string
	if err := s.db.QueryRowContext(ctx, `SELECT password_hash FROM users WHERE id = ?`, id).Scan(&hash); err != nil {
		return false
	}
	return auth.VerifyPassword(hash, password)
}

// List returns all users, oldest first.
func (s *Store) List(ctx context.Context) ([]User, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+columns+` FROM users ORDER BY created_at, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []User{}
	for rows.Next() {
		u, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// Update changes role and/or active status, keeping at least one active admin.
func (s *Store) Update(ctx context.Context, id int64, role *string, active *bool) (User, error) {
	u, err := s.ByID(ctx, id)
	if err != nil {
		return User{}, err
	}
	if role != nil {
		if *role != RoleAdmin && *role != RoleMember {
			return User{}, ErrInvalidRole
		}
		u.Role = *role
	}
	if active != nil {
		u.Active = *active
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE users SET role = ?, active = ? WHERE id = ?`, u.Role, u.Active, id); err != nil {
		return User{}, err
	}
	var admins int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE role = 'admin' AND active = 1`).Scan(&admins); err != nil {
		return User{}, err
	}
	if admins == 0 {
		return User{}, ErrLastAdmin
	}
	if !u.Active {
		if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, id); err != nil {
			return User{}, err
		}
	}
	return u, tx.Commit()
}

// SetPassword replaces a password and signs the user out everywhere.
func (s *Store) SetPassword(ctx context.Context, id int64, password string) error {
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE id = ?`, hash, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

// EnsureAdmin creates the bootstrap admin if no account uses that email yet.
func (s *Store) EnsureAdmin(ctx context.Context, name, email, password string) (bool, error) {
	_, err := s.Create(ctx, NewUser{Name: name, Email: email, Password: password, Role: RoleAdmin})
	if errors.Is(err, ErrEmailTaken) {
		return false, nil
	}
	return err == nil, err
}
