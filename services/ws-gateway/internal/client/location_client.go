package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"ws-gateway/internal/config"
)

type LocationClient interface {
	RecordLocation(ctx context.Context, driverID string, latitude, longitude float64) error
}

type locationClient struct {
	baseURL    string
	timeout    time.Duration
	httpClient *http.Client
}

type recordLocationReq struct {
	DriverID  string  `json:"driver_id"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

func NewLocationClient(cfg *config.Config) LocationClient {
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 50,
	}
	return &locationClient{
		baseURL: cfg.LocationServiceURL,
		timeout: time.Duration(cfg.LocationTimeoutMS) * time.Millisecond,
		httpClient: &http.Client{
			Transport: transport,
		},
	}
}

func (c *locationClient) RecordLocation(ctx context.Context, driverID string, latitude, longitude float64) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	payload := recordLocationReq{
		DriverID:  driverID,
		Latitude:  latitude,
		Longitude: longitude,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		slog.Error("failed to marshal location record body", "error", err)
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/v1/locations/record", bytes.NewReader(body))
	if err != nil {
		slog.Error("failed to create location record request", "error", err)
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := c.httpClient.Do(req)
	if err != nil {
		slog.Error("failed to call location record API", "driver_id", driverID, "error", err)
		return err
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, res.Body)

	if res.StatusCode != http.StatusOK {
		slog.Error("location record API returned non-200", "driver_id", driverID, "status", res.StatusCode)
		return fmt.Errorf("unexpected status: %d", res.StatusCode)
	}
	return nil
}
