package impl

import (
	"context"
	"time"

	"user-service/internal/repository"

	"github.com/redis/go-redis/v9"
)

type tokenRepositoryImpl struct {
	rdb *redis.Client
}

func NewTokenRepository(rdb *redis.Client) repository.TokenRepository {
	return &tokenRepositoryImpl{rdb: rdb}
}

func (r *tokenRepositoryImpl) SaveRefreshToken(ctx context.Context, token string, userID string, ttl time.Duration) error {
	return r.rdb.Set(ctx, "refresh:"+token, userID, ttl).Err()
}

func (r *tokenRepositoryImpl) GetDelRefreshToken(ctx context.Context, token string) (string, error) {
	val, err := r.rdb.GetDel(ctx, "refresh:"+token).Result()
	if err == redis.Nil {
		return "", nil
	}
	return val, err
}

func (r *tokenRepositoryImpl) GetRefreshToken(ctx context.Context, token string) (string, error) {
	val, err := r.rdb.Get(ctx, "refresh:"+token).Result()
	if err == redis.Nil {
		return "", nil
	}
	return val, err
}

func (r *tokenRepositoryImpl) DelRefreshToken(ctx context.Context, token string) error {
	return r.rdb.Del(ctx, "refresh:"+token).Err()
}

func (r *tokenRepositoryImpl) BlacklistToken(ctx context.Context, jti string, ttl time.Duration) error {
	return r.rdb.Set(ctx, "blacklist:"+jti, "revoked", ttl).Err()
}

func (r *tokenRepositoryImpl) GetBlacklistToken(ctx context.Context, jti string) (string, error) {
	val, err := r.rdb.Get(ctx, "blacklist:"+jti).Result()
	if err == redis.Nil {
		return "", nil
	}
	return val, err
}
