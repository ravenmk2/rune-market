package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// User roles and statuses (VARCHAR(16) + app-level validation, design §6.1).
const (
	RoleAdmin = "admin"
	RoleUser  = "user"

	StatusActive   = "active"
	StatusPending  = "pending"
	StatusDisabled = "disabled"
)

type User struct {
	ID           string
	Username     string
	Nickname     string
	PasswordHash string
	Role         string
	IsFounder    bool
	Status       string
	HasAvatar    bool
	Bio          string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type UserStore struct {
	db *sql.DB
}

func NewUserStore(db *sql.DB) *UserStore { return &UserStore{db: db} }

const userColumns = `id, username, nickname, password_hash, role, is_founder, status, has_avatar, bio, created_at, updated_at`

func (s *UserStore) Create(ctx context.Context, u *User) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO user (`+userColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		u.ID, u.Username, u.Nickname, u.PasswordHash, u.Role, u.IsFounder,
		u.Status, u.HasAvatar, u.Bio, u.CreatedAt, u.UpdatedAt)
	return err
}

func (s *UserStore) GetByID(ctx context.Context, id string) (*User, error) {
	return s.get(ctx, `SELECT `+userColumns+` FROM user WHERE id = ?`, id)
}

func (s *UserStore) GetByUsername(ctx context.Context, username string) (*User, error) {
	return s.get(ctx, `SELECT `+userColumns+` FROM user WHERE username = ?`, username)
}

func (s *UserStore) get(ctx context.Context, query string, arg any) (*User, error) {
	var u User
	err := s.db.QueryRowContext(ctx, query, arg).Scan(
		&u.ID, &u.Username, &u.Nickname, &u.PasswordHash, &u.Role,
		&u.IsFounder, &u.Status, &u.HasAvatar, &u.Bio, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}
