package repository

import "context"

type TripLink struct {
	TripID     string `json:"trip_id"`
	CustomerID string `json:"customer_id"`
}

type TripLinkReader interface {
	GetTripLink(ctx context.Context, driverID string) (*TripLink, error)
}
