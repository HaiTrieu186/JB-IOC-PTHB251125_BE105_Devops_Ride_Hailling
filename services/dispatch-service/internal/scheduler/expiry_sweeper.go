package scheduler

import (
	"context"
	"log/slog"
	"time"

	"dispatch-service/internal/service"
)

type ExpirySweeper struct {
	sweeperService service.TripSweeperService
}

func NewExpirySweeper(sweeperService service.TripSweeperService) *ExpirySweeper {
	return &ExpirySweeper{
		sweeperService: sweeperService,
	}
}

// Start khởi chạy worker quét quá hạn mỗi 2 giây, dừng khi ctx bị hủy
func (s *ExpirySweeper) Start(ctx context.Context) {
	// Quét ngay 1 lần lúc khởi động (LLD Mục 7.2 bước 3)
	if err := s.sweeperService.SweepExpiredTrips(ctx); err != nil {
		slog.Error("initial expiry sweep encountered error", "error", err)
	}

	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		slog.Info("background expiry sweeper worker started (2s interval)")

		for {
			select {
			case <-ctx.Done():
				slog.Info("stopping background expiry sweeper worker")
				return
			case <-ticker.C:
				if err := s.sweeperService.SweepExpiredTrips(ctx); err != nil {
					slog.Error("periodic expiry sweep encountered error", "error", err)
				}
			}
		}
	}()
}
