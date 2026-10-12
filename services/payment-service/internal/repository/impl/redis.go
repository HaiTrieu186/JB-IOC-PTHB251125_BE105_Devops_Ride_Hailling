package impl

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

func NewRedisClient(addr, password string) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       0,
	})

	var err error
	for attempt := 1; attempt <= 10; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err = rdb.Ping(ctx).Err()
		cancel()

		if err == nil {
			slog.Info("connected to Redis successfully", "attempt", attempt)
			return rdb, nil
		}

		slog.Warn("failed to ping Redis, retrying...", "attempt", attempt, "error", err)
		if attempt < 10 {
			time.Sleep(3 * time.Second)
		}
	}

	return nil, fmt.Errorf("could not connect to Redis after 10 attempts: %w", err)
}
