package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"ws-gateway/internal/client"
	"ws-gateway/internal/config"
	"ws-gateway/internal/consumer"
	"ws-gateway/internal/controller"
	"ws-gateway/internal/exception"
	"ws-gateway/internal/middleware"
	repoImpl "ws-gateway/internal/repository/impl"
	"ws-gateway/internal/service"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	slog.Info("starting ws-gateway...")

	cfg := config.LoadConfig()

	// 1. Kết nối Redis với cơ chế retry (LLD-00 5n)
	rdb, err := repoImpl.InitRedis(cfg)
	if err != nil {
		slog.Error("redis initialization failed", "error", err)
		os.Exit(1)
	}

	// 2. Khởi tạo repositories
	blacklistRepo := repoImpl.NewRedisBlacklistRepository(rdb)
	tripLinkRepo := repoImpl.NewRedisTripLinkRepository(rdb)

	// 3. Khởi tạo client & services
	locClient := client.NewLocationClient(cfg)
	hub := service.NewConnectionHub(cfg.MaxConnections)
	locForwarder := service.NewLocationForwarder(cfg, locClient, tripLinkRepo, hub)

	// 4. Khởi tạo controller & consumer
	wsController := controller.NewWSController(cfg, hub, locForwarder)
	pubsubSubscriber := consumer.NewPubSubSubscriber(rdb, hub)
	pubsubSubscriber.Start(context.Background())

	// 5. Khởi tạo Fiber app
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		ErrorHandler:          exception.CustomErrorHandler,
	})

	// 6. Đăng ký routes
	app.Get("/health", wsController.Health)

	handshakeMW := middleware.NewHandshakeMiddleware(cfg, blacklistRepo, hub)
	app.Get("/ws/", handshakeMW, websocket.New(wsController.HandleWS))

	// 7. Lắng nghe tín hiệu dừng (Graceful Shutdown)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		addr := ":" + cfg.Port
		slog.Info("ws-gateway listening", "port", cfg.Port)
		if err := app.Listen(addr); err != nil {
			slog.Error("HTTP server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-sigChan
	slog.Info("shutting down ws-gateway gracefully...")

	_ = app.Shutdown()
	_ = pubsubSubscriber.Stop()
	hub.CloseAll()
	_ = rdb.Close()

	slog.Info("ws-gateway stopped successfully")
}
