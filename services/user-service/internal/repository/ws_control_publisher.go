package repository

import "context"

type WSControlPublisher interface {
	PublishDisconnect(ctx context.Context, userID string) error
}
