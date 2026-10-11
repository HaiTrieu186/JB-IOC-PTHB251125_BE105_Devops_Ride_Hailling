package service

import (
	"context"

	"location-service/internal/dto"
	"location-service/internal/exception"
)

type LocationService interface {
	RecordLocation(ctx context.Context, req *dto.RecordLocationRequest) *exception.AppError
	GetCandidates(ctx context.Context, req *dto.CandidatesRequest) (*dto.CandidatesResponse, *exception.AppError)
	GetGeohashDrivers(ctx context.Context, req *dto.GeohashDriversRequest) (*dto.GeohashDriversResponse, *exception.AppError)
	GetActiveDrivers(ctx context.Context) (*dto.ActiveDriversResponse, *exception.AppError)
}
