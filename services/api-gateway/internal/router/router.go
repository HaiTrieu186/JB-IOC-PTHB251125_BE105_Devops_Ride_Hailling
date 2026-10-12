package router

import (
	"api-gateway/internal/config"
	"api-gateway/internal/exception"
	"api-gateway/internal/middleware"
	"api-gateway/internal/proxy"
	"api-gateway/internal/repository"

	"github.com/gofiber/fiber/v2"
)

type RouteEntry struct {
	Method        string
	Path          string
	TargetURL     string
	IsUserService bool
	RequireAuth   bool
	AdminOnly     bool
}

func SetupRoutes(
	app *fiber.App,
	cfg *config.Config,
	blacklistRepo repository.BlacklistRepository,
	proxyHandler *proxy.ProxyHandler,
) {
	// 1. Global Sanitize middleware (strips X-User-* and blocks /internal/*)
	app.Use(middleware.Sanitize())

	// 2. Local health check (public)
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"status": "ok",
		})
	})

	jwtAuthMiddleware := middleware.JWTAuth(cfg, blacklistRepo)
	adminGuardMiddleware := middleware.AdminGuard()

	// 3. Routing Table (all 26 public business endpoints)
	// NOTE: GET /api/v1/trips/current is registered BEFORE GET /api/v1/trips/:id
	routes := []RouteEntry{
		// user-service: public
		{Method: "POST", Path: "/api/v1/auth/register", TargetURL: cfg.UserServiceURL, IsUserService: true, RequireAuth: false},
		{Method: "POST", Path: "/api/v1/auth/login", TargetURL: cfg.UserServiceURL, IsUserService: true, RequireAuth: false},
		{Method: "POST", Path: "/api/v1/auth/refresh", TargetURL: cfg.UserServiceURL, IsUserService: true, RequireAuth: false},

		// user-service: protected
		{Method: "POST", Path: "/api/v1/auth/logout", TargetURL: cfg.UserServiceURL, IsUserService: true, RequireAuth: true, AdminOnly: false},
		{Method: "PATCH", Path: "/api/v1/driver/status", TargetURL: cfg.UserServiceURL, IsUserService: true, RequireAuth: true, AdminOnly: false},
		{Method: "GET", Path: "/api/v1/users/me", TargetURL: cfg.UserServiceURL, IsUserService: true, RequireAuth: true, AdminOnly: false},
		{Method: "PUT", Path: "/api/v1/driver/vehicle", TargetURL: cfg.UserServiceURL, IsUserService: true, RequireAuth: true, AdminOnly: false},
		{Method: "GET", Path: "/api/v1/admin/users", TargetURL: cfg.UserServiceURL, IsUserService: true, RequireAuth: true, AdminOnly: true},

		// dispatch-service: protected
		{Method: "POST", Path: "/api/v1/trips", TargetURL: cfg.DispatchServiceURL, IsUserService: false, RequireAuth: true, AdminOnly: false},
		{Method: "POST", Path: "/api/v1/trips/:id/accept", TargetURL: cfg.DispatchServiceURL, IsUserService: false, RequireAuth: true, AdminOnly: false},
		{Method: "POST", Path: "/api/v1/trips/:id/picking-up", TargetURL: cfg.DispatchServiceURL, IsUserService: false, RequireAuth: true, AdminOnly: false},
		{Method: "POST", Path: "/api/v1/trips/:id/start-trip", TargetURL: cfg.DispatchServiceURL, IsUserService: false, RequireAuth: true, AdminOnly: false},
		{Method: "POST", Path: "/api/v1/trips/:id/complete", TargetURL: cfg.DispatchServiceURL, IsUserService: false, RequireAuth: true, AdminOnly: false},
		{Method: "POST", Path: "/api/v1/trips/:id/cancel", TargetURL: cfg.DispatchServiceURL, IsUserService: false, RequireAuth: true, AdminOnly: false},
		{Method: "GET", Path: "/api/v1/trips/current", TargetURL: cfg.DispatchServiceURL, IsUserService: false, RequireAuth: true, AdminOnly: false},
		{Method: "GET", Path: "/api/v1/trips/:id", TargetURL: cfg.DispatchServiceURL, IsUserService: false, RequireAuth: true, AdminOnly: false},
		{Method: "POST", Path: "/api/v1/admin/trips/:id/force-cancel", TargetURL: cfg.DispatchServiceURL, IsUserService: false, RequireAuth: true, AdminOnly: true},
		{Method: "GET", Path: "/api/v1/admin/trips", TargetURL: cfg.DispatchServiceURL, IsUserService: false, RequireAuth: true, AdminOnly: true},

		// pricing-service: protected
		{Method: "POST", Path: "/api/v1/pricing/estimate", TargetURL: cfg.PricingServiceURL, IsUserService: false, RequireAuth: true, AdminOnly: false},
		{Method: "GET", Path: "/api/v1/admin/pricing/config", TargetURL: cfg.PricingServiceURL, IsUserService: false, RequireAuth: true, AdminOnly: true},
		{Method: "PUT", Path: "/api/v1/admin/pricing/config", TargetURL: cfg.PricingServiceURL, IsUserService: false, RequireAuth: true, AdminOnly: true},

		// payment-service: protected
		{Method: "GET", Path: "/api/v1/wallets/me", TargetURL: cfg.PaymentServiceURL, IsUserService: false, RequireAuth: true, AdminOnly: false},
		{Method: "POST", Path: "/api/v1/wallets/top-up", TargetURL: cfg.PaymentServiceURL, IsUserService: false, RequireAuth: true, AdminOnly: false},
		{Method: "GET", Path: "/api/v1/wallets/transactions", TargetURL: cfg.PaymentServiceURL, IsUserService: false, RequireAuth: true, AdminOnly: false},

		// location-service: protected
		{Method: "GET", Path: "/api/v1/admin/drivers/active", TargetURL: cfg.LocationServiceURL, IsUserService: false, RequireAuth: true, AdminOnly: true},

		// ai-service: protected
		{Method: "GET", Path: "/api/v1/admin/reports", TargetURL: cfg.AIServiceURL, IsUserService: false, RequireAuth: true, AdminOnly: true},
	}

	for _, r := range routes {
		handlers := make([]fiber.Handler, 0, 3)
		if r.RequireAuth {
			handlers = append(handlers, jwtAuthMiddleware)
		}
		if r.AdminOnly {
			handlers = append(handlers, adminGuardMiddleware)
		}
		handlers = append(handlers, proxyHandler.Forward(r.TargetURL, r.IsUserService))

		app.Add(r.Method, r.Path, handlers...)
	}

	// 4. Catch-all for unmatched routes or invalid methods -> 404 NOT_FOUND
	app.All("/*", func(c *fiber.Ctx) error {
		return exception.NewNotFoundError("Đường dẫn không tồn tại")
	})
}
