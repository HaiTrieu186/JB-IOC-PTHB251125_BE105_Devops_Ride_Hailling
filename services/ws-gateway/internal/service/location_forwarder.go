package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"ws-gateway/internal/client"
	"ws-gateway/internal/config"
	"ws-gateway/internal/repository"
)

type LocationForwarder interface {
	HandleGPS(ctx context.Context, driverID string, lat, lng float64)
}

type locationForwarder struct {
	locClient    client.LocationClient
	tripLinkRepo repository.TripLinkReader
	hub          ConnectionHub
	timeoutMS    int
}

type driverLocationUpdatedFrame struct {
	Event     string  `json:"event"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	DriverID  string  `json:"driver_id"`
	TripID    string  `json:"trip_id"`
	Timestamp string  `json:"timestamp"`
}

func NewLocationForwarder(cfg *config.Config, locClient client.LocationClient, tripLinkRepo repository.TripLinkReader, hub ConnectionHub) LocationForwarder {
	return &locationForwarder{
		locClient:    locClient,
		tripLinkRepo: tripLinkRepo,
		hub:          hub,
		timeoutMS:    cfg.LocationTimeoutMS,
	}
}

func (f *locationForwarder) HandleGPS(ctx context.Context, driverID string, lat, lng float64) {
	// (i) Goroutine riêng gọi POST /internal/v1/locations/record, lỗi chỉ log ERROR
	go func() {
		recordCtx, cancel := context.WithTimeout(context.Background(), time.Duration(f.timeoutMS)*time.Millisecond)
		defer cancel()
		_ = f.locClient.RecordLocation(recordCtx, driverID, lat, lng)
	}()

	// (ii) KHÔNG chờ (i): đọc GET trip:link:<driver_id>
	link, err := f.tripLinkRepo.GetTripLink(ctx, driverID)
	if err != nil {
		slog.Warn("failed to get trip link from redis", "driver_id", driverID, "error", err)
		return
	}
	if link == nil {
		// redis.Nil -> im lặng
		return
	}

	frame := driverLocationUpdatedFrame{
		Event:     "DRIVER_LOCATION_UPDATED",
		Latitude:  lat,
		Longitude: lng,
		DriverID:  driverID,
		TripID:    link.TripID,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	data, err := json.Marshal(frame)
	if err != nil {
		slog.Warn("failed to marshal DRIVER_LOCATION_UPDATED frame", "error", err)
		return
	}

	f.hub.PushToUser(link.CustomerID, data)
}
