package impl

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"time"

	"pricing-service/internal/client"
	"pricing-service/internal/service"
	"pricing-service/internal/util"
)

type distanceCalculatorImpl struct {
	osrmBaseURL   string
	osrmTimeoutMs time.Duration
}

func NewDistanceCalculator(osrmBaseURL string, osrmTimeoutMs int) service.DistanceCalculator {
	return &distanceCalculatorImpl{
		osrmBaseURL:   osrmBaseURL,
		osrmTimeoutMs: time.Duration(osrmTimeoutMs) * time.Millisecond,
	}
}

type osrmRoute struct {
	Distance float64 `json:"distance"`
	Duration float64 `json:"duration"`
}

type osrmResponse struct {
	Code   string      `json:"code"`
	Routes []osrmRoute `json:"routes"`
}

func (c *distanceCalculatorImpl) Calculate(ctx context.Context, pickupLat, pickupLng, dropoffLat, dropoffLng float64) (service.DistanceResult, error) {
	// 1. Thử gọi OSRM với timeout OSRM_TIMEOUT_MS
	reqCtx, cancel := context.WithTimeout(ctx, c.osrmTimeoutMs)
	defer cancel()

	osrmURL := fmt.Sprintf("%s/route/v1/driving/%f,%f;%f,%f?overview=false",
		c.osrmBaseURL, pickupLng, pickupLat, dropoffLng, dropoffLat)

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, osrmURL, nil)
	if err == nil {
		resp, doErr := client.SharedHTTPClient.Do(req)
		if doErr == nil {
			defer func() {
				_, _ = io.Copy(io.Discard, resp.Body)
				_ = resp.Body.Close()
			}()

			if resp.StatusCode == http.StatusOK {
				bodyBytes, readErr := io.ReadAll(resp.Body)
				if readErr == nil {
					var osrmRes osrmResponse
					if jsonErr := json.Unmarshal(bodyBytes, &osrmRes); jsonErr == nil && osrmRes.Code == "Ok" && len(osrmRes.Routes) > 0 {
						distMeters := int(math.Round(osrmRes.Routes[0].Distance))
						etaSec := int(math.Round(osrmRes.Routes[0].Duration))
						return service.DistanceResult{
							DistanceMeters: distMeters,
							ETASeconds:     etaSec,
							Source:         "OSRM",
						}, nil
					}
				}
			} else {
				slog.Warn("OSRM API returned non-200 status", "status", resp.StatusCode)
			}
		} else {
			slog.Warn("OSRM API request failed or timed out", "error", doErr)
		}
	} else {
		slog.Warn("failed to create OSRM request", "error", err)
	}

	// 2. Fallback sang Haversine theo Mục 4.2 LLD
	slog.Info("falling back to Haversine distance calculation")
	haversineKm := util.HaversineKm(pickupLat, pickupLng, dropoffLat, dropoffLng)

	// D_mét = floor(Haversine(pickup, dropoff) * 1.35 * 1000 + 0.5)
	distMeters := int(math.Floor(haversineKm*1.35*1000.0 + 0.5))

	// ETA_giây = floor((D_mét / 1000.0 / 30) * 3600 + 0.5)
	etaSec := int(math.Floor((float64(distMeters)/1000.0/30.0)*3600.0 + 0.5))

	return service.DistanceResult{
		DistanceMeters: distMeters,
		ETASeconds:     etaSec,
		Source:         "HAVERSINE",
	}, nil
}
