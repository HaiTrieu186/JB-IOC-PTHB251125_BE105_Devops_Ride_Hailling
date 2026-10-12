package impl

import (
	"strings"

	"dispatch-service/internal/dto"
	"dispatch-service/internal/exception"
)

func validateCoordinates(req *dto.CreateTripRequest) error {
	// Dải chuẩn LLD-00 Mục 4: vĩ độ [-85.05112878, 85.05112878], kinh độ [-180, 180]
	const minLat = -85.05112878
	const maxLat = 85.05112878
	const minLng = -180.0
	const maxLng = 180.0

	if req.PickupLat == nil || req.PickupLng == nil || req.DropoffLat == nil || req.DropoffLng == nil {
		return exception.NewValidationError("Tọa độ không được để trống")
	}

	if *req.PickupLat < minLat || *req.PickupLat > maxLat || *req.PickupLng < minLng || *req.PickupLng > maxLng {
		return exception.NewValidationError("Tọa độ điểm đón không hợp lệ")
	}
	if *req.DropoffLat < minLat || *req.DropoffLat > maxLat || *req.DropoffLng < minLng || *req.DropoffLng > maxLng {
		return exception.NewValidationError("Tọa độ điểm trả không hợp lệ")
	}

	if *req.PickupLat == *req.DropoffLat && *req.PickupLng == *req.DropoffLng {
		return exception.NewValidationError("Điểm đón và điểm trả không được trùng nhau")
	}

	if strings.TrimSpace(req.PickupAddress) == "" {
		return exception.NewValidationError("Địa chỉ đón không được để trống")
	}
	if strings.TrimSpace(req.DropoffAddress) == "" {
		return exception.NewValidationError("Địa chỉ trả không được để trống")
	}

	return nil
}
