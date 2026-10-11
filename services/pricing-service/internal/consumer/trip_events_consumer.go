package consumer

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"pricing-service/internal/config"
	"pricing-service/internal/repository"

	"github.com/redis/go-redis/v9"
)

const (
	streamName    = "stream:trip_events"
	consumerGroup = "pricing-group"
	consumerName  = "pricing-consumer-1"
)

type TripEventsConsumer struct {
	rdb                 *redis.Client
	demandRepo          repository.DemandRepository
	cfg                 *config.Config
	pelDeliveryCounts   map[string]int
	pelMu               sync.Mutex
	lastXPendingReport  sync.Once
	xpendingIncrements  bool
}

func NewTripEventsConsumer(
	rdb *redis.Client,
	demandRepo repository.DemandRepository,
	cfg *config.Config,
) *TripEventsConsumer {
	return &TripEventsConsumer{
		rdb:               rdb,
		demandRepo:        demandRepo,
		cfg:               cfg,
		pelDeliveryCounts: make(map[string]int),
	}
}

// Start khởi tạo group và chạy consumer worker loop
func (c *TripEventsConsumer) Start(ctx context.Context) {
	// 1. Khởi tạo consumer group (Idempotent)
	err := c.rdb.XGroupCreateMkStream(ctx, streamName, consumerGroup, "0").Err()
	if err != nil {
		if strings.Contains(err.Error(), "BUSYGROUP") {
			slog.Info("consumer group already exists", "group", consumerGroup)
		} else {
			slog.Warn("failed to create consumer group", "group", consumerGroup, "error", err)
		}
	} else {
		slog.Info("created consumer group", "group", consumerGroup, "stream", streamName)
	}

	// 2. Chạy vòng lặp worker tiêu thụ stream
	go c.workerLoop(ctx)
}

func (c *TripEventsConsumer) workerLoop(ctx context.Context) {
	slog.Info("trip_events consumer worker started", "group", consumerGroup, "consumer", consumerName)

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("trip_events consumer worker stopping...")
			return
		case <-ticker.C:
			// Bước A: Quét PEL ĐÚNG 1 lượt mỗi chu kỳ theo Mục 5(m) LLD-00
			c.processPEL(ctx)

			// Bước B: Đọc tin mới '>'
			c.processNewMessages(ctx)
		}
	}
}

// processPEL đọc tồn đọng PEL đúng 1 lượt
func (c *TripEventsConsumer) processPEL(ctx context.Context) {
	entries, err := c.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    consumerGroup,
		Consumer: consumerName,
		Streams:  []string{streamName, "0"},
		Count:    c.cfg.PELBatchCount,
		Block:    -1, // non-blocking
	}).Result()

	if err != nil && err != redis.Nil {
		slog.Warn("failed to read PEL messages", "error", err)
		return
	}

	if len(entries) == 0 || len(entries[0].Messages) == 0 {
		return
	}

	for _, msg := range entries[0].Messages {
		c.handlePELMessage(ctx, msg)
	}
}

func (c *TripEventsConsumer) handlePELMessage(ctx context.Context, msg redis.XMessage) {
	c.pelMu.Lock()
	count := c.pelDeliveryCounts[msg.ID] + 1
	c.pelMu.Unlock()

	// Kiểm tra xem XPENDING có tăng hay không
	c.checkXPendingDelivery(ctx, msg.ID, count)

	if int64(count) > c.cfg.MaxDeliveryAttempts {
		slog.Error("max delivery attempts exceeded in PEL, dropping and acking message",
			"msg_id", msg.ID, "attempts", count)
		c.ackMessage(ctx, msg.ID)
		return
	}

	c.pelMu.Lock()
	c.pelDeliveryCounts[msg.ID] = count
	c.pelMu.Unlock()

	c.processMessage(ctx, msg)
}

func (c *TripEventsConsumer) checkXPendingDelivery(ctx context.Context, msgID string, ramCount int) {
	pendings, err := c.rdb.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: streamName,
		Group:  consumerGroup,
		Start:  msgID,
		End:    msgID,
		Count:  1,
	}).Result()

	if err == nil && len(pendings) > 0 {
		p := pendings[0]
		slog.Debug("XPENDING check", "msg_id", msgID, "delivery_count", p.RetryCount, "ram_count", ramCount)
		c.lastXPendingReport.Do(func() {
			if p.RetryCount > 1 {
				c.xpendingIncrements = true
				slog.Info("XPENDING delivery count increments on reread", "delivered_times", p.RetryCount)
			} else {
				c.xpendingIncrements = false
				slog.Info("XPENDING delivery count does not increment on reread (using in-memory map counter)", "delivered_times", p.RetryCount)
			}
		})
	}
}

// processNewMessages đọc tin nhắn mới từ stream
func (c *TripEventsConsumer) processNewMessages(ctx context.Context) {
	entries, err := c.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    consumerGroup,
		Consumer: consumerName,
		Streams:  []string{streamName, ">"},
		Count:    c.cfg.PELBatchCount,
		Block:    2000 * time.Millisecond,
	}).Result()

	if err != nil && err != redis.Nil {
		if !strings.Contains(err.Error(), "context canceled") {
			slog.Warn("failed to read new messages from stream", "error", err)
		}
		return
	}

	if len(entries) == 0 || len(entries[0].Messages) == 0 {
		return
	}

	for _, msg := range entries[0].Messages {
		c.processMessage(ctx, msg)
	}
}

func (c *TripEventsConsumer) processMessage(ctx context.Context, msg redis.XMessage) {
	values := msg.Values
	if len(values) == 0 {
		slog.Warn("empty message entry, acking immediately", "msg_id", msg.ID)
		c.ackMessage(ctx, msg.ID)
		return
	}

	eventType, _ := values["type"].(string)
	if eventType == "" {
		slog.Warn("message missing type field, acking", "msg_id", msg.ID)
		c.ackMessage(ctx, msg.ID)
		return
	}

	// Chỉ xử lý TripCreated
	if eventType != "TripCreated" {
		c.ackMessage(ctx, msg.ID)
		return
	}

	tripID, _ := values["trip_id"].(string)
	pickupGeohash5, _ := values["pickup_geohash5"].(string)
	createdAtStr, _ := values["created_at"].(string)

	if tripID == "" || pickupGeohash5 == "" || createdAtStr == "" {
		slog.Error("TripCreated missing required fields, acking corrupted message",
			"msg_id", msg.ID, "trip_id", tripID, "geohash5", pickupGeohash5, "created_at", createdAtStr)
		c.ackMessage(ctx, msg.ID)
		return
	}

	createdAt, err := time.Parse(time.RFC3339, createdAtStr)
	if err != nil {
		slog.Error("failed to parse created_at RFC3339 timestamp, acking", "msg_id", msg.ID, "error", err)
		c.ackMessage(ctx, msg.ID)
		return
	}

	// Quy tắc lọc độ tươi: cũ hơn 5 phút (300s) -> bỏ qua không tích lũy, XACK
	diffSeconds := time.Now().UTC().Sub(createdAt).Seconds()
	if diffSeconds > 300 {
		slog.Info("TripCreated older than 300s, discarding and acking", "msg_id", msg.ID, "age_seconds", diffSeconds)
		c.ackMessage(ctx, msg.ID)
		return
	}

	// Ghi nhận vào ZSET: ZADD demand:geo:<geohash5> <unix_created_at> <trip_id>
	addErr := c.demandRepo.AddTripDemand(ctx, pickupGeohash5, tripID, createdAt.Unix())
	if addErr != nil {
		slog.Warn("temporary redis error adding trip demand, leaving in PEL", "msg_id", msg.ID, "error", addErr)
		// KHÔNG XACK, để lại trong PEL
		return
	}

	slog.Info("accumulated trip demand successfully", "trip_id", tripID, "geohash5", pickupGeohash5, "created_at", createdAtStr)
	c.ackMessage(ctx, msg.ID)
}

func (c *TripEventsConsumer) ackMessage(ctx context.Context, msgID string) {
	if err := c.rdb.XAck(ctx, streamName, consumerGroup, msgID).Err(); err != nil {
		slog.Warn("failed to XACK message", "msg_id", msgID, "error", err)
	}

	c.pelMu.Lock()
	delete(c.pelDeliveryCounts, msgID)
	c.pelMu.Unlock()
}
