package dto

import "time"

type EstimateRequest struct {
	PickupLat  *float64 `json:"pickup_lat"`
	PickupLng  *float64 `json:"pickup_lng"`
	DropoffLat *float64 `json:"dropoff_lat"`
	DropoffLng *float64 `json:"dropoff_lng"`
}

type PublicEstimateResponse struct {
	Distance        float64 `json:"distance"`
	ETA             int     `json:"eta"`
	Source          string  `json:"source"`
	SurgeMultiplier float64 `json:"surge_multiplier"`
	BaseFare        int64   `json:"base_fare"`
	PricePerKm      int64   `json:"price_per_km"`
	TotalFare       int64   `json:"total_fare"`
}

type InternalEstimateResponse struct {
	Fare            int64   `json:"fare"`
	DistanceMeters  int     `json:"distance_m"`
	ETASeconds      int     `json:"eta_seconds"`
	SurgeMultiplier float64 `json:"surge_multiplier"`
}

type AdminPricingConfigResponse struct {
	BaseFare       int64     `json:"base_fare"`
	PricePerKm     int64     `json:"price_per_km"`
	SurgeThreshold float64   `json:"surge_threshold"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type UpdatePricingConfigRequest struct {
	BaseFare       *int64   `json:"base_fare"`
	PricePerKm     *int64   `json:"price_per_km"`
	SurgeThreshold *float64 `json:"surge_threshold"`
}

type HealthResponse struct {
	Status string `json:"status"`
}
