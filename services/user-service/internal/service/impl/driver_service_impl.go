package impl

import (
	"context"
	"strings"

	"user-service/internal/dto"
	"user-service/internal/exception"
	"user-service/internal/repository"
	"user-service/internal/service"
	"user-service/internal/util"

	"github.com/google/uuid"
)

type driverServiceImpl struct {
	userRepo repository.UserRepository
}

func NewDriverService(userRepo repository.UserRepository) service.DriverService {
	return &driverServiceImpl{userRepo: userRepo}
}

func (s *driverServiceImpl) UpdateStatus(ctx context.Context, driverIDStr string, userRole string, req *dto.DriverStatusRequest) (*dto.DriverStatusResponse, *exception.AppError) {
	if userRole != "DRIVER" {
		return nil, exception.NewForbiddenError("Chỉ tài xế mới có quyền cập nhật trạng thái")
	}

	if driverIDStr == "" {
		return nil, exception.NewUnauthorizedError("Thiếu header X-User-Id")
	}

	driverUUID, err := util.ParseUUID(driverIDStr)
	if err != nil {
		return nil, exception.NewValidationError("ID tài xế không hợp lệ")
	}

	req.Status = strings.TrimSpace(req.Status)
	if req.Status != "ONLINE" && req.Status != "OFFLINE" {
		return nil, exception.NewValidationError("Trạng thái phải là ONLINE hoặc OFFLINE")
	}

	affected, dbErr := s.userRepo.UpdateDriverStatusConditional(ctx, driverUUID, req.Status)
	if dbErr != nil {
		return nil, exception.NewServiceUnavailableError("Lỗi cập nhật trạng thái tài xế")
	}

	if affected == 0 {
		return nil, exception.NewAppError(exception.ErrDriverBusy, 400, "Tài xế đang bận chuyến xe, không thể chuyển đổi trạng thái")
	}

	return &dto.DriverStatusResponse{
		DriverID: driverIDStr,
		Status:   req.Status,
	}, nil
}

func (s *driverServiceImpl) UpdateInternalStatus(ctx context.Context, driverIDStr string, req *dto.DriverInternalStatusRequest) (*dto.DriverStatusResponse, *exception.AppError) {
	driverUUID, err := util.ParseUUID(driverIDStr)
	if err != nil {
		return nil, exception.NewValidationError("ID tài xế không phải UUID hợp lệ")
	}

	req.FromStatus = strings.TrimSpace(req.FromStatus)
	req.ToStatus = strings.TrimSpace(req.ToStatus)

	isValidPair := (req.FromStatus == "ONLINE" && req.ToStatus == "BUSY") || (req.FromStatus == "BUSY" && req.ToStatus == "ONLINE")
	if !isValidPair {
		return nil, exception.NewValidationError("Cặp chuyển đổi trạng thái không hợp lệ")
	}

	affected, dbErr := s.userRepo.UpdateDriverStatusInternalConditional(ctx, driverUUID, req.FromStatus, req.ToStatus)
	if dbErr != nil {
		return nil, exception.NewServiceUnavailableError("Lỗi cập nhật trạng thái tài xế")
	}

	if affected == 0 {
		return nil, exception.NewAppError(exception.ErrDriverNotAvailable, 400, "Trạng thái hiện tại khác from_status hoặc không tìm thấy tài xế")
	}

	return &dto.DriverStatusResponse{
		DriverID: driverIDStr,
		Status:   req.ToStatus,
	}, nil
}

func (s *driverServiceImpl) FilterOnline(ctx context.Context, req *dto.FilterOnlineRequest) (*dto.FilterOnlineResponse, *exception.AppError) {
	if req.DriverIDs == nil {
		return nil, exception.NewValidationError("Dữ liệu driver_ids không hợp lệ")
	}

	if len(req.DriverIDs) == 0 {
		return &dto.FilterOnlineResponse{
			OnlineDriverIDs: []string{},
		}, nil
	}

	if len(req.DriverIDs) > 500 {
		return nil, exception.NewValidationError("Mảng driver_ids vượt quá giới hạn tối đa 500 phần tử")
	}

	parsedUUIDs := make([]uuid.UUID, 0, len(req.DriverIDs))
	for _, idStr := range req.DriverIDs {
		id, err := util.ParseUUID(idStr)
		if err != nil {
			return nil, exception.NewValidationError("Phần tử không phải UUID hợp lệ: " + idStr)
		}
		parsedUUIDs = append(parsedUUIDs, id)
	}

	onlineIDs, err := s.userRepo.FindOnlineDriverIDs(ctx, parsedUUIDs)
	if err != nil {
		return nil, exception.NewServiceUnavailableError("Lỗi truy vấn cơ sở dữ liệu")
	}

	return &dto.FilterOnlineResponse{
		OnlineDriverIDs: onlineIDs,
	}, nil
}
