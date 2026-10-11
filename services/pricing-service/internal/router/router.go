package router

import (
	"pricing-service/internal/controller"
	"pricing-service/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupRoutes(
	app *fiber.App,
	healthCtrl *controller.HealthController,
	publicCtrl *controller.PublicController,
	adminCtrl *controller.AdminController,
	internalCtrl *controller.InternalController,
) {
	// 1. Healthcheck
	app.Get("/health", healthCtrl.Health)

	// 2. Public API (yêu cầu đăng nhập X-User-Id & X-User-Role)
	api := app.Group("/api/v1")
	pricing := api.Group("/pricing")
	pricing.Post("/estimate", middleware.RequireAuth(), publicCtrl.Estimate)

	// 3. Admin API (yêu cầu X-User-Role == "ADMIN")
	adminPricing := api.Group("/admin/pricing", middleware.RequireAdmin())
	adminPricing.Get("/config", adminCtrl.GetConfig)
	adminPricing.Put("/config", adminCtrl.UpdateConfig)

	// 4. Internal API (chỉ gọi nội bộ giữa các service)
	internal := app.Group("/internal/v1")
	internalPricing := internal.Group("/pricing")
	internalPricing.Post("/estimate", internalCtrl.Estimate)
}
