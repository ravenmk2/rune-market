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
	db DBTX
}

func NewUserStore(db DBTX) *UserStore { return &UserStore{db: db} }

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

// UserFilter carries the admin user-list query parameters.
type UserFilter struct {
	Status   string // "" = any
	Query    string // username/nickname LIKE
	Page     int
	PageSize int
}

// List returns users matching the filter plus the total count.
func (s *UserStore) List(ctx context.Context, f UserFilter) ([]*User, int, error) {
	where := []string{"1=1"}
	args := []any{}
	if f.Status != "" {
		where = append(where, `status = ?`)
		args = append(args, f.Status)
	}
	if f.Query != "" {
		where = append(where, `(username LIKE ? OR nickname LIKE ?)`)
		like := "%" + f.Query + "%"
		args = append(args, like, like)
	}
	cond := " WHERE " + where[0]
	for _, w := range where[1:] {
		cond += " AND " + w
	}

	var total int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM user`+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	page := f.Page
	if page < 1 {
		page = 1
	}
	pageSize := f.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+userColumns+` FROM user`+cond+` ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		append(args, pageSize, (page-1)*pageSize)...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	var out []*User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.Nickname, &u.PasswordHash,
			&u.Role, &u.IsFounder, &u.Status, &u.HasAvatar, &u.Bio,
			&u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, &u)
	}
	return out, total, rows.Err()
}

// Count returns the total number of users.
func (s *UserStore) Count(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM user`).Scan(&n)
	return n, err
}

func (s *UserStore) UpdateStatus(ctx context.Context, id, status string, updatedAt time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE user SET status = ?, updated_at = ? WHERE id = ?`, status, updatedAt, id)
	return err
}

func (s *UserStore) UpdateRole(ctx context.Context, id, role string, updatedAt time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE user SET role = ?, updated_at = ? WHERE id = ?`, role, updatedAt, id)
	return err
}

func (s *UserStore) UpdatePassword(ctx context.Context, id, passwordHash string, updatedAt time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE user SET password_hash = ?, updated_at = ? WHERE id = ?`, passwordHash, updatedAt, id)
	return err
}

// Delete removes the user row; sessions cascade via FK.
func (s *UserStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM user WHERE id = ?`, id)
	return err
}
