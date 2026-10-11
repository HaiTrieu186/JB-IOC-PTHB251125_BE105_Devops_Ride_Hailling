package service

import (
	"context"

	"pricing-service/internal/dto"
)

type PricingService interface {
	EstimatePublic(ctx context.Context, req *dto.EstimateRequest) (*dto.PublicEstimateResponse, error)
	EstimateInternal(ctx context.Context, req *dto.EstimateRequest) (*dto.InternalEstimateResponse, error)
}
