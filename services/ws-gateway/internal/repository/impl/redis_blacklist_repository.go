package impl

import (
	"context"
	"fmt"
	"time"

	"ws-gateway/internal/repository"

	"github.com/redis/go-redis/v9"
)

type redisBlacklistRepository struct {
	rdb *redis.Client
}

func NewRedisBlacklistRepository(rdb *redis.Client) repository.BlacklistReader {
	return &redisBlacklistRepository{rdb: rdb}
}

func (r *redisBlacklistRepository) IsBlacklisted(ctx context.Context, jti string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	_, err := r.rdb.Get(ctx, fmt.Sprintf("blacklist:%s", jti)).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
