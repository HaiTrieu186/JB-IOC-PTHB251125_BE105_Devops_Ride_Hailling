package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"location-service/internal/config"
	"location-service/internal/controller"
	"location-service/internal/exception"
	"location-service/internal/middleware"
	repoImpl "location-service/internal/repository/impl"
	"location-service/internal/router"
	serviceImpl "location-service/internal/service/impl"

	"github.com/gofiber/fiber/v2"
)

func main() {
	// Logger JSON ra stdout
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	slog.Info("starting location-service...")

	cfg := config.LoadConfig()

	// 1. Kết nối Redis qua impl
	rdb, err := repoImpl.InitRedis(cfg)
	if err != nil {
		slog.Error("redis initialization failed", "error", err)
		os.Exit(1)
	}

	// 2. Khởi tạo Repositories
	locationRepo := repoImpl.NewLocationRepository(rdb)

	// 3. Khởi tạo Services
	locationService := serviceImpl.NewLocationService(locationRepo, cfg)

	// 4. Khởi tạo Controllers
	healthCtrl := controller.NewHealthController()
	publicCtrl := controller.NewPublicController(locationService)
	internalCtrl := controller.NewInternalController(locationService)

	// 5. Cấu hình Fiber App với ErrorHandler duy nhất
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		ErrorHandler:          exception.CustomErrorHandler,
	})

	app.Use(middleware.Logger())

	// 6. Thiết lập Routes
	router.SetupRoutes(app, healthCtrl, publicCtrl, internalCtrl)

	// In danh sách route thật đang đăng ký (đối chiếu LLD)
	for _, r := range app.GetRoutes(true) {
		slog.Info("registered route", "method", r.Method, "path", r.Path)
	}

	// 7. Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		addr := ":" + cfg.Port
		slog.Info("location-service HTTP server listening", "port", cfg.Port)
		if err := app.Listen(addr); err != nil {
			slog.Info("HTTP server stopped", "error", err)
		}
	}()

	<-sigChan
	slog.Info("shutting down location-service gracefully...")

	_ = app.Shutdown()
	_ = rdb.Close()

	slog.Info("location-service stopped successfully")
}
