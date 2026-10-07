// Package auth implements registration, login, sessions and the permission
// middleware (design §7). Interfaces are defined here (consumer side); the
// store package satisfies them implicitly.
package auth

import (
	"context"
	"time"

	"github.com/ravenmk2/rune-market/internal/store"
)

type UserRepository interface {
	Create(ctx context.Context, u *store.User) error
	GetByUsername(ctx context.Context, username string) (*store.User, error)
	GetByID(ctx context.Context, id string) (*store.User, error)
}

type SessionRepository interface {
	Create(ctx context.Context, s *store.Session) error
	GetByTokenHash(ctx context.Context, tokenHash string) (*store.Session, error)
	DeleteByTokenHash(ctx context.Context, tokenHash string) error
	Touch(ctx context.Context, id string, expiresAt time.Time) error
}

type SettingRepository interface {
	Get(ctx context.Context, key string) (string, error)
}
