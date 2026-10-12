package dto

import (
	"time"

	"github.com/google/uuid"
)

type CreateTripRequest struct {
	PickupLat      float64 `json:"pickup_lat"`
	PickupLng      float64 `json:"pickup_lng"`
	PickupAddress  string  `json:"pickup_address"`
	DropoffLat     float64 `json:"dropoff_lat"`
	DropoffLng     float64 `json:"dropoff_lng"`
	DropoffAddress string  `json:"dropoff_address"`
}

type CreateTripResponse struct {
	ID                uuid.UUID `json:"id"`
	Status            string    `json:"status"`
	Fare              int64     `json:"fare"`
	PickupAddress     string    `json:"pickup_address"`
	DropoffAddress    string    `json:"dropoff_address"`
	MatchingExpiresAt string    `json:"matching_expires_at"`
}

type AcceptTripResponse struct {
	ID       uuid.UUID `json:"id"`
	Status   string    `json:"status"`
	DriverID uuid.UUID `json:"driver_id"`
}

type StatusResponse struct {
	ID     uuid.UUID `json:"id"`
	Status string    `json:"status"`
}

type CompleteTripResponse struct {
	ID          uuid.UUID `json:"id"`
	Status      string    `json:"status"`
	CompletedAt string    `json:"completed_at"`
}

type CancelTripRequest struct {
	Reason *string `json:"reason"`
}

type ForceCancelRequest struct {
	Reason string `json:"reason"`
}

type ForceCancelResponse struct {
	ID          uuid.UUID `json:"id"`
	Status      string    `json:"status"`
	CancelledBy string    `json:"cancelled_by"`
}

type TripResponse struct {
	ID                uuid.UUID  `json:"id"`
	CustomerID        uuid.UUID  `json:"customer_id"`
	DriverID          *uuid.UUID `json:"driver_id"`
	Status            string     `json:"status"`
	PickupLat         float64    `json:"pickup_lat"`
	PickupLng         float64    `json:"pickup_lng"`
	PickupAddress     string     `json:"pickup_address"`
	DropoffLat        float64    `json:"dropoff_lat"`
	DropoffLng        float64    `json:"dropoff_lng"`
	DropoffAddress    string     `json:"dropoff_address"`
	Fare              int64      `json:"fare"`
	DistanceM         int        `json:"distance_m"`
	SurgeMultiplier   float64    `json:"surge_multiplier"`
	PickupGeohash5    string     `json:"pickup_geohash5"`
	MatchingExpiresAt string     `json:"matching_expires_at"`
	CompletedAt       *string    `json:"completed_at"`
	CancelledBy       *string    `json:"cancelled_by"`
	CancelReason      *string    `json:"cancel_reason"`
	CreatedAt         string     `json:"created_at"`
	UpdatedAt         string     `json:"updated_at"`
}

type TimelineResponse struct {
	ID        int64   `json:"id"`
	TripID    string  `json:"trip_id"`
	Status    string  `json:"status"`
	Note      *string `json:"note"`
	CreatedAt string  `json:"created_at"`
}

type TripDetailResponse struct {
	Trip           TripResponse       `json:"trip"`
	StatusTimeline []TimelineResponse `json:"status_timeline"`
}

type AdminTripsResponse struct {
	Trips []TripResponse `json:"trips"`
	Total int64          `json:"total"`
	Page  int            `json:"page"`
}

func FormatTimeUTC(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

func FormatTimeUTCPtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format(time.RFC3339)
	return &s
}
