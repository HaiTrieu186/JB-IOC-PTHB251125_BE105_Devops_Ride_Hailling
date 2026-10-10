package impl

import (
	"context"
	"encoding/json"

	"user-service/internal/repository"

	"github.com/redis/go-redis/v9"
)

type wsControlPublisherImpl struct {
	rdb *redis.Client
}

func NewWSControlPublisher(rdb *redis.Client) repository.WSControlPublisher {
	return &wsControlPublisherImpl{rdb: rdb}
}

func (p *wsControlPublisherImpl) PublishDisconnect(ctx context.Context, userID string) error {
	payload, err := json.Marshal(map[string]string{
		"action":  "DISCONNECT",
		"user_id": userID,
	})
	if err != nil {
		return err
	}
	return p.rdb.Publish(ctx, "ride:ws_control", string(payload)).Err()
}
