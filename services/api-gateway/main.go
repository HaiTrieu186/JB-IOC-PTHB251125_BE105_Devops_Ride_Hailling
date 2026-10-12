package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"api-gateway/internal/config"
	"api-gateway/internal/exception"
	"api-gateway/internal/proxy"
	repoImpl "api-gateway/internal/repository/impl"
	"api-gateway/internal/router"

	"github.com/gofiber/fiber/v2"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	slog.Info("starting api-gateway...")

	cfg := config.LoadConfig()

	// 1. Kết nối Redis với cơ chế retry (LLD-00 5n)
	rdb, err := repoImpl.InitRedis(cfg)
	if err != nil {
		slog.Error("redis initialization failed", "error", err)
		os.Exit(1)
	}

	// 2. Khởi tạo repository & handler
	blacklistRepo := repoImpl.NewRedisBlacklistRepository(rdb)
	proxyHandler := proxy.NewProxyHandler(cfg)

	// 3. Khởi tạo Fiber app với ErrorHandler duy nhất
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		ErrorHandler:          exception.CustomErrorHandler,
	})

	// 4. Thiết lập Routes
	router.SetupRoutes(app, cfg, blacklistRepo, proxyHandler)

	for _, r := range app.GetRoutes(true) {
		slog.Info("registered route", "method", r.Method, "path", r.Path)
	}

	// 5. Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		addr := ":" + cfg.Port
		slog.Info("api-gateway HTTP server listening", "port", cfg.Port)
		if err := app.Listen(addr); err != nil {
			slog.Error("HTTP server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-sigChan
	slog.Info("shutting down api-gateway gracefully...")

	_ = app.Shutdown()
	_ = rdb.Close()

	slog.Info("api-gateway stopped successfully")
}
