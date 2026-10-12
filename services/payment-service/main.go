package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"

	"payment-service/internal/config"
	"payment-service/internal/consumer"
	"payment-service/internal/controller"
	"payment-service/internal/exception"
	"payment-service/internal/middleware"
	repoImpl "payment-service/internal/repository/impl"
	"payment-service/internal/router"
	svcImpl "payment-service/internal/service/impl"
)

func main() {
	// 1. Structured JSON logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	slog.Info("starting payment-service...")

	// 2. Configuration
	cfg := config.LoadConfig()

	// 3. PostgreSQL connection & auto migrations
	db, err := repoImpl.NewPostgresDB(cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to PostgreSQL paymentdb", "error", err)
		os.Exit(1)
	}

	// 4. Redis connection
	rdb, err := repoImpl.NewRedisClient(cfg.RedisAddr, cfg.RedisPassword)
	if err != nil {
		slog.Error("failed to connect to Redis", "error", err)
		os.Exit(1)
	}
	defer rdb.Close()

	// 5. Dependency Injection: Repository layer
	walletRepo := repoImpl.NewWalletRepository(db)
	transactionRepo := repoImpl.NewTransactionRepository(db)
	transactor := repoImpl.NewTransactor(db)

	// 6. Dependency Injection: Service layer
	walletService := svcImpl.NewWalletService(walletRepo, transactionRepo, cfg.TopupMaxAmount)
	settlementService := svcImpl.NewSettlementService(transactor, transactionRepo, cfg.CommissionRate)

	// 7. Dependency Injection: Controller layer
	walletCtrl := controller.NewWalletController(walletService)

	// 8. Stream Consumer startup
	consumerCtx, cancelConsumer := context.WithCancel(context.Background())
	defer cancelConsumer()

	tripConsumer := consumer.NewTripCompletedConsumer(
		rdb,
		settlementService,
		cfg.MaxDeliveryAttempts,
		cfg.PELBatchCount,
	)
	if err := tripConsumer.Start(consumerCtx); err != nil {
		slog.Error("failed to start trip_completed consumer", "error", err)
		os.Exit(1)
	}

	// 9. Fiber App initialization
	app := fiber.New(fiber.Config{
		ErrorHandler: exception.ErrorHandler,
	})

	app.Use(middleware.LoggerMiddleware())

	// 10. Route registration
	router.SetupRoutes(app, walletCtrl)

	// 11. Graceful shutdown
	go func() {
		addr := ":" + cfg.Port
		slog.Info("payment-service HTTP server listening", "port", cfg.Port)
		if err := app.Listen(addr); err != nil {
			slog.Info("payment-service HTTP server stopped", "error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down payment-service gracefully...")
	cancelConsumer()

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()

	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		slog.Error("error shutting down fiber app", "error", err)
	}

	slog.Info("payment-service exited cleanly")
}
