package consumer

import (
	"context"
	"encoding/json"
	"log/slog"

	"ws-gateway/internal/service"

	"github.com/gofiber/contrib/websocket"
	"github.com/redis/go-redis/v9"
)

type PubSubSubscriber interface {
	Start(ctx context.Context)
	Stop() error
}

type pubSubSubscriber struct {
	rdb    *redis.Client
	hub    service.ConnectionHub
	pubsub *redis.PubSub
}

func NewPubSubSubscriber(rdb *redis.Client, hub service.ConnectionHub) PubSubSubscriber {
	return &pubSubSubscriber{
		rdb: rdb,
		hub: hub,
	}
}

type tripOfferTarget struct {
	DriverID  string `json:"driver_id"`
	DistanceM int    `json:"distance_m"`
}

type tripOfferMsg struct {
	TripID     string            `json:"trip_id"`
	Targets    []tripOfferTarget `json:"targets"`
	PickupLat  float64           `json:"pickup_lat"`
	PickupLng  float64           `json:"pickup_lng"`
	Fare       int64             `json:"fare"`
	DriverFare int64             `json:"driver_fare"`
	ExpireAt   string            `json:"expire_at"`
}

type tripOfferedFrame struct {
	Event      string  `json:"event"`
	TripID     string  `json:"trip_id"`
	PickupLat  float64 `json:"pickup_lat"`
	PickupLng  float64 `json:"pickup_lng"`
	Fare       int64   `json:"fare"`
	DriverFare int64   `json:"driver_fare"`
	DistanceM  int     `json:"distance_m"`
	ExpireAt   string  `json:"expire_at"`
}

type tripUpdateMsg struct {
	TripID     string  `json:"trip_id"`
	CustomerID string  `json:"customer_id"`
	DriverID   *string `json:"driver_id"`
	Status     string  `json:"status"`
	UpdatedAt  string  `json:"updated_at"`
}

type tripStatusUpdatedFrame struct {
	Event     string `json:"event"`
	TripID    string `json:"trip_id"`
	Status    string `json:"status"`
	UpdatedAt string `json:"updated_at"`
}

type wsControlMsg struct {
	Action string `json:"action"`
	UserID string `json:"user_id"`
}

func (s *pubSubSubscriber) Start(ctx context.Context) {
	s.pubsub = s.rdb.Subscribe(ctx, "ride:trip_offers", "ride:trip_updates", "ride:ws_control")
	ch := s.pubsub.Channel()

	go func() {
		slog.Info("redis pub/sub subscriber started listening")
		for msg := range ch {
			switch msg.Channel {
			case "ride:trip_offers":
				s.handleTripOffers(msg.Payload)
			case "ride:trip_updates":
				s.handleTripUpdates(msg.Payload)
			case "ride:ws_control":
				s.handleWSControl(msg.Payload)
			}
		}
		slog.Info("redis pub/sub subscriber stopped")
	}()
}

func (s *pubSubSubscriber) Stop() error {
	if s.pubsub != nil {
		return s.pubsub.Close()
	}
	return nil
}

func (s *pubSubSubscriber) handleTripOffers(payload string) {
	var offer tripOfferMsg
	if err := json.Unmarshal([]byte(payload), &offer); err != nil {
		slog.Warn("failed to parse ride:trip_offers payload", "error", err)
		return
	}

	for _, target := range offer.Targets {
		frame := tripOfferedFrame{
			Event:      "TRIP_OFFERED",
			TripID:     offer.TripID,
			PickupLat:  offer.PickupLat,
			PickupLng:  offer.PickupLng,
			Fare:       offer.Fare,
			DriverFare: offer.DriverFare,
			DistanceM:  target.DistanceM,
			ExpireAt:   offer.ExpireAt,
		}
		data, err := json.Marshal(frame)
		if err != nil {
			slog.Warn("failed to marshal TRIP_OFFERED frame", "error", err)
			continue
		}
		s.hub.PushToUser(target.DriverID, data)
	}
}

func (s *pubSubSubscriber) handleTripUpdates(payload string) {
	var update tripUpdateMsg
	if err := json.Unmarshal([]byte(payload), &update); err != nil {
		slog.Warn("failed to parse ride:trip_updates payload", "error", err)
		return
	}

	frame := tripStatusUpdatedFrame{
		Event:     "TRIP_STATUS_UPDATED",
		TripID:    update.TripID,
		Status:    update.Status,
		UpdatedAt: update.UpdatedAt,
	}
	data, err := json.Marshal(frame)
	if err != nil {
		slog.Warn("failed to marshal TRIP_STATUS_UPDATED frame", "error", err)
		return
	}

	if update.CustomerID != "" {
		s.hub.PushToUser(update.CustomerID, data)
	}

	if update.DriverID != nil && *update.DriverID != "" {
		s.hub.PushToUser(*update.DriverID, data)
	}
}

func (s *pubSubSubscriber) handleWSControl(payload string) {
	var ctrl wsControlMsg
	if err := json.Unmarshal([]byte(payload), &ctrl); err != nil {
		slog.Warn("failed to parse ride:ws_control payload", "error", err)
		return
	}

	if ctrl.Action == "DISCONNECT" && ctrl.UserID != "" {
		slog.Info("disconnecting user due to ws_control", "user_id", ctrl.UserID)
		s.hub.DisconnectUser(ctrl.UserID, websocket.CloseNormalClosure, "DISCONNECT")
	}
}
