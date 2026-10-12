package repository

import "context"

type BlacklistRepository interface {
	IsBlacklisted(ctx context.Context, jti string) (bool, error)
}
