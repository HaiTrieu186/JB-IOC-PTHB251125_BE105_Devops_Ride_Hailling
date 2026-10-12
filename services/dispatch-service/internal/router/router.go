package router

import (
	"github.com/gofiber/fiber/v2"

	"dispatch-service/internal/controller"
	"dispatch-service/internal/middleware"
)

func SetupRoutes(
	app *fiber.App,
	healthCtrl *controller.HealthController,
	tripCtrl *controller.TripController,
	adminCtrl *controller.AdminTripController,
) {
	// 1. Healthcheck endpoint
	app.Get("/health", healthCtrl.HealthCheck)

	// 2. Trips API group
	api := app.Group("/api/v1/trips")

	// Đặt /current TRƯỚC /:id để tránh nhầm "current" là tham số :id (LLD Mục 3)
	api.Post("/", middleware.RequireCustomer(), tripCtrl.CreateTrip)
	api.Get("/current", middleware.RequireCustomerOrDriver(), tripCtrl.GetCurrentTrip)

	api.Post("/:id/accept", middleware.RequireDriver(), tripCtrl.AcceptTrip)
	api.Post("/:id/picking-up", middleware.RequireDriver(), tripCtrl.PickingUp)
	api.Post("/:id/start-trip", middleware.RequireDriver(), tripCtrl.StartTrip)
	api.Post("/:id/complete", middleware.RequireDriver(), tripCtrl.CompleteTrip)
	api.Post("/:id/cancel", middleware.RequireCustomerOrDriver(), tripCtrl.CancelTrip)
	api.Get("/:id", middleware.RequireCustomerOrDriver(), tripCtrl.GetTripDetail)

	// 3. Admin Trips API group
	admin := app.Group("/api/v1/admin/trips", middleware.RequireAdmin())
	admin.Get("/", adminCtrl.ListTrips)
	admin.Post("/:id/force-cancel", adminCtrl.ForceCancel)
}
