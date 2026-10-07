package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type Session struct {
	ID        string
	UserID    string
	TokenHash string // sha256 of the opaque token; the token itself is never stored
	IP        string
	UserAgent string
	ExpiresAt time.Time
	CreatedAt time.Time
}

type SessionStore struct {
	db DBTX
}

func NewSessionStore(db DBTX) *SessionStore { return &SessionStore{db: db} }

func (s *SessionStore) Create(ctx context.Context, sess *Session) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO session (id, user_id, token_hash, ip, user_agent, expires_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		sess.ID, sess.UserID, sess.TokenHash, sess.IP, sess.UserAgent,
		sess.ExpiresAt, sess.CreatedAt)
	return err
}

func (s *SessionStore) GetByTokenHash(ctx context.Context, tokenHash string) (*Session, error) {
	var sess Session
	err := s.db.QueryRowContext(ctx,
		`SELECT id, user_id, token_hash, ip, user_agent, expires_at, created_at
		 FROM session WHERE token_hash = ?`, tokenHash).
		Scan(&sess.ID, &sess.UserID, &sess.TokenHash, &sess.IP, &sess.UserAgent,
			&sess.ExpiresAt, &sess.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &sess, nil
}

func (s *SessionStore) DeleteByTokenHash(ctx context.Context, tokenHash string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM session WHERE token_hash = ?`, tokenHash)
	return err
}

// Touch extends the expiry of a live session (sliding renewal).
func (s *SessionStore) Touch(ctx context.Context, id string, expiresAt time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE session SET expires_at = ? WHERE id = ?`, expiresAt, id)
	return err
}

// DeleteExpired removes expired sessions; safe to call periodically.
func (s *SessionStore) DeleteExpired(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM session WHERE expires_at < ?`, Now())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
