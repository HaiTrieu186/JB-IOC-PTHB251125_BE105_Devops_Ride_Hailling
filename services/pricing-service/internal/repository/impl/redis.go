package impl

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"pricing-service/internal/config"

	"github.com/redis/go-redis/v9"
)

func InitRedis(cfg *config.Config) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       0,
	})

	var err error
	for attempt := 1; attempt <= 10; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_, err = rdb.Ping(ctx).Result()
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
