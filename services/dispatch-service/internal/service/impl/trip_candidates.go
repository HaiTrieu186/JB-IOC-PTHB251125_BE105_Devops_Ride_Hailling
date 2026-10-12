package impl

import (
	"context"
	"log/slog"
	"sort"

	"github.com/google/uuid"

	"dispatch-service/internal/client"
	"dispatch-service/internal/exception"
)

func (s *tripCreateServiceImpl) findMatchedCandidates(ctx context.Context, tripID uuid.UUID, pickupLat, pickupLng float64) ([]client.DriverCandidate, error) {
	candidates, err := s.locationClient.GetCandidates(ctx, pickupLat, pickupLng, s.offerRadiusKM)
	if err != nil {
		slog.Error("failed to get candidates from location-service", "trip_id", tripID, "error", err)
		return nil, exception.NewServiceUnavailableError("location-service không phản hồi hoặc timeout", err)
	}

	var matchedCandidates []client.DriverCandidate
	if len(candidates) > 0 {
		driverIDs := make([]string, len(candidates))
		for i, c := range candidates {
			driverIDs[i] = c.DriverID
		}

		onlineIDs, err := s.userClient.FilterOnline(ctx, driverIDs)
		if err != nil {
			slog.Error("failed to filter online drivers from user-service", "trip_id", tripID, "error", err)
			return nil, exception.NewServiceUnavailableError("user-service không phản hồi hoặc timeout", err)
		}

		if len(onlineIDs) > 0 {
			onlineMap := make(map[string]bool, len(onlineIDs))
			for _, id := range onlineIDs {
				onlineMap[id] = true
			}

			for _, c := range candidates {
				if onlineMap[c.DriverID] {
					matchedCandidates = append(matchedCandidates, c)
				}
			}

			// Sắp xếp tăng dần theo khoảng cách distance_m (Quyết định D)
			sort.Slice(matchedCandidates, func(i, j int) bool {
				return matchedCandidates[i].DistanceM < matchedCandidates[j].DistanceM
			})

			// Top-N: min(số tài xế online khớp, 5) (Quyết định A.3)
			if len(matchedCandidates) > 5 {
				matchedCandidates = matchedCandidates[:5]
			}
		}
	}

	return matchedCandidates, nil
}
