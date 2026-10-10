package router

import (
	"user-service/internal/controller"
	"user-service/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupRoutes(
	app *fiber.App,
	healthCtrl *controller.HealthController,
	authCtrl *controller.AuthController,
	driverCtrl *controller.DriverController,
	internalCtrl *controller.InternalController,
) {
	// Healthcheck
	app.Get("/health", healthCtrl.Health)

	// Public Auth & User Endpoints
	api := app.Group("/api/v1")
	api.Post("/auth/register", authCtrl.Register)
	api.Post("/auth/login", authCtrl.Login)
	api.Post("/auth/refresh", authCtrl.Refresh)
	api.Post("/auth/logout", middleware.AuthRequired(), authCtrl.Logout)
	api.Patch("/driver/status", middleware.RequireRoles("DRIVER"), driverCtrl.UpdateDriverStatus)

	// Internal Endpoints
	internal := app.Group("/internal/v1")
	internal.Post("/users/filter-online", internalCtrl.FilterOnline)
	internal.Post("/drivers/:id/status", internalCtrl.UpdateDriverInternalStatus)
}
