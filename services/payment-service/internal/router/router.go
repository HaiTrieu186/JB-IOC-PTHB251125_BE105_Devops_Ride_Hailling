package router

import (
	"github.com/gofiber/fiber/v2"

	"payment-service/internal/controller"
	"payment-service/internal/middleware"
	"payment-service/internal/util"
)

func SetupRoutes(app *fiber.App, walletCtrl *controller.WalletController) {
	// 1. Healthcheck
	app.Get("/health", func(c *fiber.Ctx) error {
		return util.SuccessResponse(c, fiber.StatusOK, fiber.Map{
			"status": "ok",
		})
	})

	// 2. Public Wallet API (Requires CUSTOMER or DRIVER, forbids ADMIN)
	walletGroup := app.Group("/api/v1/wallets", middleware.RequireWalletUser())
	walletGroup.Get("/me", walletCtrl.GetMe)
	walletGroup.Post("/top-up", walletCtrl.TopUp)
	walletGroup.Get("/transactions", walletCtrl.GetTransactions)

	// 3. Internal Wallet API (dispatch-service)
	internalGroup := app.Group("/internal/v1/wallets")
	internalGroup.Post("/check-balance", walletCtrl.CheckBalance)
}
