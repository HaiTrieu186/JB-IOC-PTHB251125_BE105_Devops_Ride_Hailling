package controller

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"payment-service/internal/dto"
	"payment-service/internal/exception"
	"payment-service/internal/service"
	"payment-service/internal/util"
)

type WalletController struct {
	walletService service.WalletService
}

func NewWalletController(walletService service.WalletService) *WalletController {
	return &WalletController{walletService: walletService}
}

func (ctrl *WalletController) TopUp(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(uuid.UUID)
	if !ok {
		return exception.NewUnauthorizedError("Yêu cầu xác thực tài khoản")
	}

	var req dto.TopUpRequest
	if err := c.BodyParser(&req); err != nil {
		return exception.NewValidationError("Dữ liệu JSON không hợp lệ")
	}

	res, appErr := ctrl.walletService.TopUp(c.Context(), userID, req.Amount)
	if appErr != nil {
		return appErr
	}

	return util.SuccessResponse(c, fiber.StatusOK, res)
}

func (ctrl *WalletController) GetMe(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(uuid.UUID)
	if !ok {
		return exception.NewUnauthorizedError("Yêu cầu xác thực tài khoản")
	}

	res, appErr := ctrl.walletService.GetMe(c.Context(), userID)
	if appErr != nil {
		return appErr
	}

	return util.SuccessResponse(c, fiber.StatusOK, res)
}

func (ctrl *WalletController) GetTransactions(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(uuid.UUID)
	if !ok {
		return exception.NewUnauthorizedError("Yêu cầu xác thực tài khoản")
	}

	pageStr := c.Query("page", "1")
	page, err := strconv.Atoi(pageStr)
	if err != nil {
		return exception.NewValidationError("page phải là số nguyên")
	}

	limitStr := c.Query("limit", "20")
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		return exception.NewValidationError("limit phải là số nguyên")
	}

	res, appErr := ctrl.walletService.GetTransactions(c.Context(), userID, page, limit)
	if appErr != nil {
		return appErr
	}

	return util.SuccessResponse(c, fiber.StatusOK, res)
}

func (ctrl *WalletController) CheckBalance(c *fiber.Ctx) error {
	var req dto.CheckBalanceRequest
	if err := c.BodyParser(&req); err != nil {
		return exception.NewValidationError("Dữ liệu JSON không hợp lệ")
	}

	res, appErr := ctrl.walletService.CheckBalance(c.Context(), req)
	if appErr != nil {
		return appErr
	}

	return util.SuccessResponse(c, fiber.StatusOK, res)
}
