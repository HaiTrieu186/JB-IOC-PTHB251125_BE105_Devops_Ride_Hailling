package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v2"

	"dispatch-service/internal/client"
	"dispatch-service/internal/config"
	"dispatch-service/internal/controller"
	"dispatch-service/internal/exception"
	"dispatch-service/internal/middleware"
	repoImpl "dispatch-service/internal/repository/impl"
	"dispatch-service/internal/router"
	"dispatch-service/internal/scheduler"
	svcImpl "dispatch-service/internal/service/impl"
)

func main() {
	// 1. Structured JSON logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	slog.Info("starting dispatch-service...")

	// 2. Configuration
	cfg := config.LoadConfig()

	// 3. PostgreSQL connection & auto migrations (LLD Mục 7.2 bước 1)
	db, err := repoImpl.NewPostgresDB(cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to PostgreSQL dispatchdb", "error", err)
		os.Exit(1)
	}

	// 4. Redis connection (LLD Mục 7.2 bước 2)
	rdb, err := repoImpl.NewRedisClient(cfg.RedisAddr, cfg.RedisPassword)
	if err != nil {
		slog.Error("failed to connect to Redis", "error", err)
		os.Exit(1)
	}
	defer rdb.Close()

	// 5. Dependency Injection: Repository layer
	tripRepo := repoImpl.NewTripRepository(db)
	timelineRepo := repoImpl.NewTimelineRepository(db)
	lockRepo := repoImpl.NewLockRepository(rdb)
	tripLinkRepo := repoImpl.NewTripLinkRepository(rdb)
	eventPublisher := repoImpl.NewEventPublisher(rdb)
	transactor := repoImpl.NewTransactor(db)

	// 6. Dependency Injection: Client layer
	pricingClient := client.NewPricingClient(cfg.PricingServiceURL, cfg.InternalHTTPTimeoutMS)
	paymentClient := client.NewPaymentClient(cfg.PaymentServiceURL, cfg.InternalHTTPTimeoutMS)
	locationClient := client.NewLocationClient(cfg.LocationServiceURL, cfg.InternalHTTPTimeoutMS)
	userClient := client.NewUserClient(cfg.UserServiceURL, cfg.InternalHTTPTimeoutMS)

	// 7. Dependency Injection: Service layer
	tripCreateService := svcImpl.NewTripCreateService(
		tripRepo,
		timelineRepo,
		transactor,
		eventPublisher,
		pricingClient,
		paymentClient,
		locationClient,
		userClient,
		cfg.OfferRadiusKM,
		cfg.MatchingTimeoutSeconds,
		cfg.CommissionRate,
	)
	tripAcceptService := svcImpl.NewTripAcceptService(
		tripRepo,
		timelineRepo,
		lockRepo,
		tripLinkRepo,
		eventPublisher,
		userClient,
	)
	tripProgressService := svcImpl.NewTripProgressService(
		tripRepo,
		timelineRepo,
		tripLinkRepo,
		eventPublisher,
		userClient,
	)
	tripSweeperService := svcImpl.NewTripSweeperService(
		tripRepo,
		timelineRepo,
		eventPublisher,
	)

	// 8. Dependency Injection: Controller layer
	healthCtrl := controller.NewHealthController()
	tripCtrl := controller.NewTripController(tripCreateService, tripAcceptService, tripProgressService)
	adminCtrl := controller.NewAdminTripController(tripProgressService)

	// 9. Sweeper Worker startup (LLD Mục 7.2 bước 3 & 4)
	sweeperCtx, cancelSweeper := context.WithCancel(context.Background())
	defer cancelSweeper()

	expirySweeper := scheduler.NewExpirySweeper(tripSweeperService)
	expirySweeper.Start(sweeperCtx)

	// 10. Fiber App initialization (LLD Mục 7.2 bước 5)
	app := fiber.New(fiber.Config{
		ErrorHandler: exception.ErrorHandler,
	})

	app.Use(middleware.LoggerMiddleware())

	// 11. Route registration
	router.SetupRoutes(app, healthCtrl, tripCtrl, adminCtrl)

	// 12. Listen server & Graceful shutdown
	go func() {
		addr := ":" + cfg.Port
		slog.Info("dispatch-service HTTP server listening", "port", cfg.Port)
		if err := app.Listen(addr); err != nil {
			slog.Info("dispatch-service HTTP server stopped", "error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down dispatch-service gracefully...")
	cancelSweeper()

	if err := app.Shutdown(); err != nil {
		slog.Error("error during fiber app shutdown", "error", err)
	}

	slog.Info("dispatch-service exited cleanly")
}
