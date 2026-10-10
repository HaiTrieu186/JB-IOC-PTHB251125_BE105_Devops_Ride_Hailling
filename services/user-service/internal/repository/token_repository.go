package repository

import (
	"context"
	"time"
)

type TokenRepository interface {
	SaveRefreshToken(ctx context.Context, token string, userID string, ttl time.Duration) error
	GetDelRefreshToken(ctx context.Context, token string) (string, error)
	GetRefreshToken(ctx context.Context, token string) (string, error)
	DelRefreshToken(ctx context.Context, token string) error
	BlacklistToken(ctx context.Context, jti string, ttl time.Duration) error
	GetBlacklistToken(ctx context.Context, jti string) (string, error)
}
