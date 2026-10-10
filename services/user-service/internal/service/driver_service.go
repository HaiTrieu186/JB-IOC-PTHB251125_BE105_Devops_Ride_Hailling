package service

import (
	"context"

	"user-service/internal/dto"
	"user-service/internal/exception"
)

type DriverService interface {
	UpdateStatus(ctx context.Context, driverIDStr string, userRole string, req *dto.DriverStatusRequest) (*dto.DriverStatusResponse, *exception.AppError)
	UpdateInternalStatus(ctx context.Context, driverIDStr string, req *dto.DriverInternalStatusRequest) (*dto.DriverStatusResponse, *exception.AppError)
	FilterOnline(ctx context.Context, req *dto.FilterOnlineRequest) (*dto.FilterOnlineResponse, *exception.AppError)
}
