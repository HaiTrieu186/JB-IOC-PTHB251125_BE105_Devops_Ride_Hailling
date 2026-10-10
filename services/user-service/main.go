package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"user-service/internal/config"
	"user-service/internal/controller"
	"user-service/internal/exception"
	"user-service/internal/middleware"
	repoImpl "user-service/internal/repository/impl"
	"user-service/internal/router"
	serviceImpl "user-service/internal/service/impl"

	"github.com/gofiber/fiber/v2"
)

func main() {
	// Logger JSON ra stdout
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	slog.Info("starting user-service...")

	cfg := config.LoadConfig()

	// 1. Kết nối DB và Redis qua impl
	db, err := repoImpl.InitDB(cfg)
	if err != nil {
		slog.Error("database initialization failed", "error", err)
		os.Exit(1)
	}

	rdb, err := repoImpl.InitRedis(cfg)
	if err != nil {
		slog.Error("redis initialization failed", "error", err)
		os.Exit(1)
	}

	// 2. Khởi tạo Repositories
	userRepo := repoImpl.NewUserRepository(db)
	tokenRepo := repoImpl.NewTokenRepository(rdb)
	wsPublisher := repoImpl.NewWSControlPublisher(rdb)

	// 3. Khởi tạo Services
	authService := serviceImpl.NewAuthService(userRepo, tokenRepo, wsPublisher, cfg)
	driverService := serviceImpl.NewDriverService(userRepo)

	// 4. Seed Admin tự động (FR-30, Idempotent)
	if err := authService.SeedAdmin(context.Background()); err != nil {
		slog.Error("admin seeding failed", "error", err)
		os.Exit(1)
	}

	// 5. Khởi tạo Controllers
	healthCtrl := controller.NewHealthController()
	authCtrl := controller.NewAuthController(authService)
	driverCtrl := controller.NewDriverController(driverService)
	internalCtrl := controller.NewInternalController(driverService)

	// 6. Cấu hình Fiber App với ErrorHandler duy nhất
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		ErrorHandler:          exception.CustomErrorHandler,
	})

	app.Use(middleware.Logger())

	// 7. Thiết lập Routes
	router.SetupRoutes(app, healthCtrl, authCtrl, driverCtrl, internalCtrl)

	// In danh sách route thật đang đăng ký (đối chiếu LLD)
	for _, r := range app.GetRoutes(true) {
		slog.Info("registered route", "method", r.Method, "path", r.Path)
	}

	// 8. Graceful shutdown
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
