package impl

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"user-service/internal/config"

	"github.com/redis/go-redis/v9"
)

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
