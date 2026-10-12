package repository

import "context"

type BlacklistReader interface {
	IsBlacklisted(ctx context.Context, jti string) (bool, error)
}
