package controller

import (
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"dispatch-service/internal/dto"
	"dispatch-service/internal/exception"
	"dispatch-service/internal/service"
	"dispatch-service/internal/util"
)

type TripController struct {
	createService   service.TripCreateService
	acceptService   service.TripAcceptService
	progressService service.TripProgressService
}

func NewTripController(
	createService service.TripCreateService,
	acceptService service.TripAcceptService,
	progressService service.TripProgressService,
) *TripController {
	return &TripController{
		createService:   createService,
		acceptService:   acceptService,
		progressService: progressService,
	}
}

func (ctrl *TripController) CreateTrip(c *fiber.Ctx) error {
	customerID := c.Locals("user_id").(uuid.UUID)

	var req dto.CreateTripRequest
	if err := c.BodyParser(&req); err != nil {
		return exception.NewValidationError("Dữ liệu tạo chuyến xe không hợp lệ")
	}

	res, err := ctrl.createService.CreateTrip(c.Context(), customerID, &req)
	if err != nil {
		return err
	}

	return util.SuccessResponse(c, http.StatusCreated, res)
}

func (ctrl *TripController) AcceptTrip(c *fiber.Ctx) error {
	driverID := c.Locals("user_id").(uuid.UUID)

	tripID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return exception.NewValidationError("Mã chuyến xe không hợp lệ")
	}

	res, err := ctrl.acceptService.AcceptTrip(c.Context(), driverID, tripID)
	if err != nil {
		return err
	}

	return util.SuccessResponse(c, http.StatusOK, res)
}

func (ctrl *TripController) PickingUp(c *fiber.Ctx) error {
	driverID := c.Locals("user_id").(uuid.UUID)

	tripID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return exception.NewValidationError("Mã chuyến xe không hợp lệ")
	}

	res, err := ctrl.progressService.PickingUp(c.Context(), driverID, tripID)
	if err != nil {
		return err
	}

	return util.SuccessResponse(c, http.StatusOK, res)
}

func (ctrl *TripController) StartTrip(c *fiber.Ctx) error {
	driverID := c.Locals("user_id").(uuid.UUID)

	tripID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return exception.NewValidationError("Mã chuyến xe không hợp lệ")
	}

	res, err := ctrl.progressService.StartTrip(c.Context(), driverID, tripID)
	if err != nil {
		return err
	}

	return util.SuccessResponse(c, http.StatusOK, res)
}

func (ctrl *TripController) CompleteTrip(c *fiber.Ctx) error {
	driverID := c.Locals("user_id").(uuid.UUID)

	tripID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return exception.NewValidationError("Mã chuyến xe không hợp lệ")
	}

	res, err := ctrl.progressService.CompleteTrip(c.Context(), driverID, tripID)
	if err != nil {
		return err
	}

	return util.SuccessResponse(c, http.StatusOK, res)
}

func (ctrl *TripController) CancelTrip(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(uuid.UUID)
	role := c.Locals("user_role").(string)

	tripID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return exception.NewValidationError("Mã chuyến xe không hợp lệ")
	}

	var req dto.CancelTripRequest
	_ = c.BodyParser(&req)

	res, err := ctrl.progressService.CancelTrip(c.Context(), userID, role, tripID, req.Reason)
	if err != nil {
		return err
	}

	return util.SuccessResponse(c, http.StatusOK, res)
}

func (ctrl *TripController) GetCurrentTrip(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(uuid.UUID)
	role := c.Locals("user_role").(string)

	res, err := ctrl.progressService.GetCurrentTrip(c.Context(), userID, role)
	if err != nil {
		return err
	}

	return util.SuccessResponse(c, http.StatusOK, res)
}

func (ctrl *TripController) GetTripDetail(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(uuid.UUID)
	role := c.Locals("user_role").(string)

	tripID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return exception.NewValidationError("Mã chuyến xe không hợp lệ")
	}

	res, err := ctrl.progressService.GetTripDetail(c.Context(), userID, role, tripID)
	if err != nil {
		return err
	}

	return util.SuccessResponse(c, http.StatusOK, res)
}
