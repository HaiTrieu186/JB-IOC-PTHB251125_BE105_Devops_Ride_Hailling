package router

import (
	"location-service/internal/controller"
	"location-service/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupRoutes(
	app *fiber.App,
	healthCtrl *controller.HealthController,
	publicCtrl *controller.PublicController,
	internalCtrl *controller.InternalController,
) {
	// Health check
	app.Get("/health", healthCtrl.HealthCheck)

	// API Công khai (Public Endpoints)
	api := app.Group("/api/v1")
	admin := api.Group("/admin", middleware.RequireAdmin())
	admin.Get("/drivers/active", publicCtrl.GetActiveDrivers)

	// API Nội bộ (Internal Endpoints)
	internal := app.Group("/internal/v1/locations")
	internal.Post("/record", internalCtrl.RecordLocation)
	internal.Post("/candidates", internalCtrl.GetCandidates)
	internal.Post("/geohash-drivers", internalCtrl.GetGeohashDrivers)
}
