package consumer

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"payment-service/internal/service"
)

type TripCompletedConsumer struct {
	rdb                 *redis.Client
	settlementService   service.SettlementService
	streamName          string
	groupName           string
	consumerName        string
	maxDeliveryAttempts int
	pelBatchCount       int64
	pelDeliveryCounts   map[string]int
	mu                  sync.Mutex
}

func NewTripCompletedConsumer(
	rdb *redis.Client,
	settlementService service.SettlementService,
	maxDeliveryAttempts int,
	pelBatchCount int64,
) *TripCompletedConsumer {
	return &TripCompletedConsumer{
		rdb:                 rdb,
		settlementService:   settlementService,
		streamName:          "stream:trip_events",
		groupName:           "payment-group",
		consumerName:        "payment-consumer-1",
		maxDeliveryAttempts: maxDeliveryAttempts,
		pelBatchCount:       pelBatchCount,
		pelDeliveryCounts:   make(map[string]int),
	}
}

func (c *TripCompletedConsumer) Start(ctx context.Context) error {
	// Initialize stream consumer group with MKSTREAM
	err := c.rdb.XGroupCreateMkStream(ctx, c.streamName, c.groupName, "0").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return fmt.Errorf("failed to create consumer group %s: %w", c.groupName, err)
	}

	slog.Info("trip_completed consumer group initialized",
		"stream", c.streamName,
		"group", c.groupName,
		"consumer", c.consumerName,
	)

	// Background worker
	go c.runLoop(ctx)
	return nil
}

func (c *TripCompletedConsumer) runLoop(ctx context.Context) {
	slog.Info("trip_completed consumer worker started",
		"group", c.groupName,
		"consumer", c.consumerName,
	)

	// Periodic timer for scanning PEL once per cycle (every 3 seconds)
	pelTicker := time.NewTicker(3 * time.Second)
	defer pelTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("stopping trip_completed consumer worker")
			return
		case <-pelTicker.C:
			c.processPendingMessages(ctx)
		default:
			// Read new messages from stream with BLOCK 2000
			c.processNewMessages(ctx)
		}
	}
}

func (c *TripCompletedConsumer) processPendingMessages(ctx context.Context) {
	// Read pending messages in PEL exactly 1 batch per cycle per LLD-00 Section 5(m)
	entries, err := c.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    c.groupName,
		Consumer: c.consumerName,
		Streams:  []string{c.streamName, "0"},
		Count:    c.pelBatchCount,
		Block:    0,
	}).Result()

	if err != nil {
		if err != redis.Nil && ctx.Err() == nil {
			slog.Error("error reading PEL from stream", "error", err)
		}
		return
	}

	for _, stream := range entries {
		for _, msg := range stream.Messages {
			c.mu.Lock()
			count := c.pelDeliveryCounts[msg.ID]
			// Lần giao đầu qua '>' không được đếm trong PEL, nên lần đầu tin xuất hiện trong PEL tính là lần 2
			if count == 0 {
				count = 1
			}
			count++
			c.pelDeliveryCounts[msg.ID] = count
			c.mu.Unlock()

			if count > c.maxDeliveryAttempts {
				slog.Error("TripCompleted exceeded MAX_DELIVERY_ATTEMPTS in PEL, acking and discarding message",
					"msg_id", msg.ID,
					"attempts", count,
					"max", c.maxDeliveryAttempts,
				)
				_ = c.rdb.XAck(ctx, c.streamName, c.groupName, msg.ID).Err()
				c.mu.Lock()
				delete(c.pelDeliveryCounts, msg.ID)
				c.mu.Unlock()
				continue
			}

			if err := c.processMessage(ctx, msg); err == nil {
				_ = c.rdb.XAck(ctx, c.streamName, c.groupName, msg.ID).Err()
				c.mu.Lock()
				delete(c.pelDeliveryCounts, msg.ID)
				c.mu.Unlock()
			}
		}
	}
}

func (c *TripCompletedConsumer) processNewMessages(ctx context.Context) {
	entries, err := c.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    c.groupName,
		Consumer: c.consumerName,
		Streams:  []string{c.streamName, ">"},
		Count:    c.pelBatchCount,
		Block:    2 * time.Second,
	}).Result()

	if err != nil {
		if err != redis.Nil && ctx.Err() == nil {
			slog.Error("error reading new messages from stream", "error", err)
		}
		return
	}

	for _, stream := range entries {
		for _, msg := range stream.Messages {
			if err := c.processMessage(ctx, msg); err == nil {
				_ = c.rdb.XAck(ctx, c.streamName, c.groupName, msg.ID).Err()
				c.mu.Lock()
				delete(c.pelDeliveryCounts, msg.ID)
				c.mu.Unlock()
			}
		}
	}
}

func (c *TripCompletedConsumer) processMessage(ctx context.Context, msg redis.XMessage) error {
	// Empty message check (entry deleted by MAXLEN when reading PEL)
	if len(msg.Values) == 0 {
		slog.Warn("skipping and acking empty stream entry", "msg_id", msg.ID)
		_ = c.rdb.XAck(ctx, c.streamName, c.groupName, msg.ID).Err()
		return nil
	}

	msgType, _ := msg.Values["type"].(string)
	if msgType != "TripCompleted" {
		// Ignore events meant for other services (TripCreated, TripCancelled, TripExpired)
		_ = c.rdb.XAck(ctx, c.streamName, c.groupName, msg.ID).Err()
		return nil
	}

	tripIDStr, okTrip := msg.Values["trip_id"].(string)
	customerIDStr, okCust := msg.Values["customer_id"].(string)
	driverIDStr, okDriver := msg.Values["driver_id"].(string)
	fareVal, okFare := msg.Values["fare"]

	if !okTrip || !okCust || !okDriver || !okFare ||
		tripIDStr == "" || customerIDStr == "" || driverIDStr == "" {
		slog.Error("corrupted TripCompleted message: missing required fields, acking and discarding",
			"msg_id", msg.ID,
		)
		_ = c.rdb.XAck(ctx, c.streamName, c.groupName, msg.ID).Err()
		return nil
	}

	tripUUID, errTrip := uuid.Parse(tripIDStr)
	customerUUID, errCust := uuid.Parse(customerIDStr)
	driverUUID, errDriver := uuid.Parse(driverIDStr)
	if errTrip != nil || errCust != nil || errDriver != nil {
		slog.Error("corrupted TripCompleted message: invalid UUID format, acking and discarding",
			"msg_id", msg.ID,
			"trip_id", tripIDStr,
			"customer_id", customerIDStr,
			"driver_id", driverIDStr,
		)
		_ = c.rdb.XAck(ctx, c.streamName, c.groupName, msg.ID).Err()
		return nil
	}

	fare, errFare := parseFare(fareVal)
	if errFare != nil || fare <= 0 {
		slog.Error("corrupted TripCompleted message: invalid fare value, acking and discarding",
			"msg_id", msg.ID,
			"fare", fareVal,
		)
		_ = c.rdb.XAck(ctx, c.streamName, c.groupName, msg.ID).Err()
		return nil
	}

	// Process settlement business logic
	return c.settlementService.ProcessTripCompleted(ctx, tripUUID, customerUUID, driverUUID, fare)
}

func parseFare(val interface{}) (int64, error) {
	switch v := val.(type) {
	case string:
		return strconv.ParseInt(v, 10, 64)
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case float64:
		return int64(v), nil
	default:
		return 0, fmt.Errorf("unsupported fare type: %T", val)
	}
}
