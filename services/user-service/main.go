package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
)

func main() {
	// Khởi tạo logger JSON ra stdout theo quy ước hệ thống
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	slog.Info("starting user-service...")

	cfg := loadConfig()

	// 1. Khởi tạo DB & Migration
	db, err := initDB(cfg)
	if err != nil {
		slog.Error("database initialization failed", "error", err)
		os.Exit(1)
	}

	// 2. Khởi tạo Redis
	rdb, err := initRedis(cfg)
	if err != nil {
		slog.Error("redis initialization failed", "error", err)
		os.Exit(1)
	}

	// 3. Khởi tạo Admin tự động (FR-30, Idempotent)
	if err := seedAdmin(db, cfg); err != nil {
		slog.Error("admin seeding failed", "error", err)
		os.Exit(1)
	}

	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	// JSON request logging middleware
	app.Use(func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		duration := time.Since(start)

		slog.Info("http_request",
			"method", c.Method(),
			"path", c.Path(),
			"status", c.Response().StatusCode(),
			"duration_ms", duration.Milliseconds(),
			"ip", c.IP(),
		)
		return err
	})

	handler := newAppHandler(db, rdb, cfg)

	// Healthcheck
	app.Get("/health", handler.Health)

	// Public Auth & User Endpoints
	api := app.Group("/api/v1")
	api.Post("/auth/register", handler.Register)
	api.Post("/auth/login", handler.Login)
	api.Post("/auth/refresh", handler.Refresh)
	api.Post("/auth/logout", handler.Logout)
	api.Patch("/driver/status", handler.UpdateDriverStatus)

	// Internal Endpoints
	internal := app.Group("/internal/v1")
	internal.Post("/users/filter-online", handler.FilterOnline)
	internal.Post("/drivers/:id/status", handler.UpdateDriverInternalStatus)

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		addr := ":" + cfg.Port
		slog.Info("user-service HTTP server listening", "port", cfg.Port)
		if err := app.Listen(addr); err != nil {
			slog.Info("HTTP server stopped", "error", err)
		}
	}()

	<-sigChan
	slog.Info("shutting down user-service gracefully...")

	_ = app.Shutdown()

	sqlDB, err := db.DB()
	if err == nil {
		_ = sqlDB.Close()
	}
	_ = rdb.Close()

	slog.Info("user-service stopped successfully")
}
