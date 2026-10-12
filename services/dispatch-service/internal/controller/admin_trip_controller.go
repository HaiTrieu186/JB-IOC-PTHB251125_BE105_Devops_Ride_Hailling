package controller

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"dispatch-service/internal/dto"
	"dispatch-service/internal/exception"
	"dispatch-service/internal/service"
	"dispatch-service/internal/util"
)

type AdminTripController struct {
	progressService service.TripProgressService
}

func NewAdminTripController(progressService service.TripProgressService) *AdminTripController {
	return &AdminTripController{
		progressService: progressService,
	}
}

func (ctrl *AdminTripController) ForceCancel(c *fiber.Ctx) error {
	tripID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return exception.NewValidationError("Mã chuyến xe không hợp lệ")
	}

	var req dto.ForceCancelRequest
	if err := c.BodyParser(&req); err != nil {
		return exception.NewValidationError("Dữ liệu force-cancel không hợp lệ")
	}

	if strings.TrimSpace(req.Reason) == "" {
		return exception.NewValidationError("Lý do hủy là bắt buộc đối với quản trị viên")
	}

	res, err := ctrl.progressService.ForceCancelTrip(c.Context(), tripID, req.Reason)
	if err != nil {
		return err
	}

	return util.SuccessResponse(c, http.StatusOK, res)
}

func (ctrl *AdminTripController) ListTrips(c *fiber.Ctx) error {
	page := 1
	limit := 20

	pageStr := c.Query("page")
	if pageStr != "" {
		p, err := strconv.Atoi(pageStr)
		if err != nil || p < 1 {
			return exception.NewValidationError("Tham số page phải là số nguyên dương lớn hơn hoặc bằng 1")
		}
		page = p
	}

	limitStr := c.Query("limit")
	if limitStr != "" {
		l, err := strconv.Atoi(limitStr)
		if err != nil || l < 1 || l > 100 {
			return exception.NewValidationError("Tham số limit phải là số nguyên từ 1 đến 100")
		}
		limit = l
	}

	status := strings.TrimSpace(c.Query("status"))
	if status != "" {
		validStatuses := map[string]bool{
			"CREATED":    true,
			"MATCHING":   true,
			"ACCEPTED":   true,
			"PICKING_UP": true,
			"IN_TRIP":    true,
			"COMPLETED":  true,
			"CANCELLED":  true,
			"EXPIRED":    true,
		}
		if !validStatuses[status] {
			return exception.NewValidationError("Trạng thái chuyến xe cần lọc không hợp lệ")
		}
	}

	res, err := ctrl.progressService.ListTripsAdmin(c.Context(), status, page, limit)
	if err != nil {
		return err
	}

	return util.SuccessResponse(c, http.StatusOK, res)
}
