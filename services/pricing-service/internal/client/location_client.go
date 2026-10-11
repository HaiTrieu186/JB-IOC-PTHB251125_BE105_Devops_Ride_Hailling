package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type LocationClient interface {
	GetGeohashDrivers(ctx context.Context, geohash5 string) ([]string, error)
}

type locationClientImpl struct {
	baseURL string
	timeout time.Duration
}

func NewLocationClient(baseURL string, timeoutMs int) LocationClient {
	return &locationClientImpl{
		baseURL: baseURL,
		timeout: time.Duration(timeoutMs) * time.Millisecond,
	}
}

type geohashDriversRequest struct {
	Geohash5 string `json:"geohash5"`
}

type geohashDriversResponse struct {
	Success bool `json:"success"`
	Data    struct {
		DriverIDs []string `json:"driver_ids"`
	} `json:"data"`
	Error any `json:"error"`
}

func (c *locationClientImpl) GetGeohashDrivers(ctx context.Context, geohash5 string) ([]string, error) {
	reqCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	payload, err := json.Marshal(geohashDriversRequest{Geohash5: geohash5})
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/internal/v1/locations/geohash-drivers", c.baseURL)
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := SharedHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("location-service returned status %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var res geohashDriversResponse
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return nil, err
	}

	return res.Data.DriverIDs, nil
}
