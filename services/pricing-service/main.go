package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"pricing-service/internal/client"
	"pricing-service/internal/config"
	"pricing-service/internal/consumer"
	"pricing-service/internal/controller"
	"pricing-service/internal/entity"
	"pricing-service/internal/exception"
	"pricing-service/internal/middleware"
	repoImpl "pricing-service/internal/repository/impl"
	"pricing-service/internal/router"
	serviceImpl "pricing-service/internal/service/impl"

	"github.com/gofiber/fiber/v2"
)

func main() {
	// Logger JSON ra stdout
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	slog.Info("starting pricing-service...")

	// 1. Tải và kiểm tra cấu hình ENV (bao gồm kiểm tra SURGE_THRESHOLD)
	cfg, err := config.LoadConfig()
	if err != nil {
		slog.Error("failed to load configuration or invalid SURGE_THRESHOLD", "error", err)
		os.Exit(1)
	}

	// 2. Kết nối PostgreSQL và AutoMigrate + Constraints
	db, err := repoImpl.InitDB(cfg)
	if err != nil {
		slog.Error("postgres initialization failed", "error", err)
		os.Exit(1)
	}

	// 3. Khởi tạo PricingConfigRepository và seed ENV nếu DB rỗng (Mục 7.2 LLD)
	configRepo := repoImpl.NewPricingConfigRepository(db)
	count, err := configRepo.Count(context.Background())
	if err != nil {
		slog.Error("failed to check existing pricing config records", "error", err)
		os.Exit(1)
	}

	if count == 0 {
		slog.Info("pricing_configs table is empty, seeding default config from ENV",
			"base_fare", cfg.DefaultBaseFare,
			"price_per_km", cfg.DefaultPricePerKm,
			"surge_threshold", cfg.SurgeThreshold)

		seedErr := configRepo.SeedIfNotExists(context.Background(), &entity.PricingConfig{
			ID:             1,
			BaseFare:       cfg.DefaultBaseFare,
			PricePerKm:     cfg.DefaultPricePerKm,
			SurgeThreshold: cfg.SurgeThreshold,
		})
		if seedErr != nil {
			slog.Error("failed to seed default pricing config", "error", seedErr)
			os.Exit(1)
		}
	} else {
		slog.Info("pricing_configs record already exists in database, skipping seed")
	}

	// 4. Kết nối Redis
	rdb, err := repoImpl.InitRedis(cfg)
	if err != nil {
		slog.Error("redis initialization failed", "error", err)
		os.Exit(1)
	}

	// 5. Khởi tạo các Repositories khác
	cacheRepo := repoImpl.NewConfigCacheRepository(rdb)
	demandRepo := repoImpl.NewDemandRepository(rdb)

	// 6. Khởi tạo Clients
	locationClient := client.NewLocationClient(cfg.LocationServiceURL, cfg.InternalHTTPTimeoutMs)
	userClient := client.NewUserClient(cfg.UserServiceURL, cfg.InternalHTTPTimeoutMs)

	// 7. Khởi tạo Services
	distanceCalc := serviceImpl.NewDistanceCalculator(cfg.OSRMBaseURL, cfg.OSRMTimeoutMs)
	surgeService := serviceImpl.NewSurgeService(demandRepo, locationClient, userClient)
	pricingConfigService := serviceImpl.NewPricingConfigService(configRepo, cacheRepo)
	pricingService := serviceImpl.NewPricingService(pricingConfigService, distanceCalc, surgeService)

	// 8. Khởi tạo Consumer và chạy goroutine nền
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tripConsumer := consumer.NewTripEventsConsumer(rdb, demandRepo, cfg)
	tripConsumer.Start(ctx)

	// 9. Khởi tạo Controllers
	healthCtrl := controller.NewHealthController()
	publicCtrl := controller.NewPublicController(pricingService)
	adminCtrl := controller.NewAdminController(pricingConfigService)
	internalCtrl := controller.NewInternalController(pricingService)

	// 10. Cấu hình Fiber App với Custom ErrorHandler
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		ErrorHandler:          exception.CustomErrorHandler,
	})

	app.Use(middleware.Logger())

	// 11. Đăng ký Routes
	router.SetupRoutes(app, healthCtrl, publicCtrl, adminCtrl, internalCtrl)

	// In danh sách route thật đang đăng ký
	for _, r := range app.GetRoutes(true) {
		slog.Info("registered route", "method", r.Method, "path", r.Path)
	}

	// 12. Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		addr := ":" + cfg.Port
		slog.Info("pricing-service HTTP server listening", "port", cfg.Port)
		if err := app.Listen(addr); err != nil {
			slog.Info("HTTP server stopped", "error", err)
		}
	}()

	<-sigChan
	slog.Info("shutting down pricing-service gracefully...")

	cancel() // dừng consumer worker
	_ = app.Shutdown()
	_ = rdb.Close()
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}

	slog.Info("pricing-service stopped successfully")
}
