package impl

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"api-gateway/internal/config"
	"api-gateway/internal/repository"

	"github.com/redis/go-redis/v9"
)

type redisBlacklistRepository struct {
	rdb *redis.Client
}

func NewRedisBlacklistRepository(rdb *redis.Client) repository.BlacklistRepository {
	return &redisBlacklistRepository{rdb: rdb}
}

func (r *redisBlacklistRepository) IsBlacklisted(ctx context.Context, jti string) (bool, error) {
	val, err := r.rdb.Get(ctx, fmt.Sprintf("blacklist:%s", jti)).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return val != "", nil
}

func InitRedis(cfg *config.Config) (*redis.Client, error) {
	var rdb *redis.Client
	var err error

	for attempt := 1; attempt <= 10; attempt++ {
		rdb = redis.NewClient(&redis.Options{
			Addr:     cfg.RedisAddr,
			Password: cfg.RedisPassword,
		})
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err = rdb.Ping(ctx).Err()
		cancel()

		if err == nil {
			slog.Info("connected to redis successfully")
			return rdb, nil
		}
		slog.Warn("failed to connect to redis, retrying...", "attempt", attempt, "max", 10, "error", err)
		time.Sleep(3 * time.Second)
	}

	return nil, fmt.Errorf("unable to connect to redis after 10 attempts: %w", err)
}
