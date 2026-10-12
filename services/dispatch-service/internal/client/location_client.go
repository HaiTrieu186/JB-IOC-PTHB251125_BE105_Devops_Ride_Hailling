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

	"dispatch-service/internal/exception"
)

type DriverCandidate struct {
	DriverID  string `json:"driver_id"`
	DistanceM int    `json:"distance_m"`
}

type LocationClient interface {
	GetCandidates(ctx context.Context, pickupLat, pickupLng, radiusKM float64) ([]DriverCandidate, error)
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

type candidatesRequest struct {
	PickupLat float64 `json:"pickup_lat"`
	PickupLng float64 `json:"pickup_lng"`
	RadiusKM  float64 `json:"radius_km"`
}

type candidatesEnvelope struct {
	Success bool `json:"success"`
	Data    *struct {
		Candidates []DriverCandidate `json:"candidates"`
	} `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (c *locationClientImpl) GetCandidates(ctx context.Context, pickupLat, pickupLng, radiusKM float64) ([]DriverCandidate, error) {
	reqCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	payload, err := json.Marshal(candidatesRequest{
		PickupLat: pickupLat,
		PickupLng: pickupLng,
		RadiusKM:  radiusKM,
	})
	if err != nil {
		return nil, exception.NewServiceUnavailableError("Lỗi đóng gói request tới location-service", err)
	}

	url := fmt.Sprintf("%s/internal/v1/locations/candidates", c.baseURL)
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, exception.NewServiceUnavailableError("Lỗi khởi tạo request tới location-service", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := SharedHTTPClient.Do(req)
	if err != nil {
		slog.Error("location-service network error / timeout", "error", err)
		return nil, exception.NewServiceUnavailableError("location-service không phản hồi hoặc timeout", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, exception.NewServiceUnavailableError("Lỗi đọc body từ location-service", err)
	}

	var envelope candidatesEnvelope
	if err := json.Unmarshal(bodyBytes, &envelope); err != nil {
		return nil, exception.NewServiceUnavailableError("Lỗi giải mã response từ location-service", err)
	}

	if resp.StatusCode == http.StatusBadRequest {
		slog.Error("location-service returned 400 (invalid request from dispatch)", "body", string(bodyBytes))
		return nil, exception.NewServiceUnavailableError("location-service trả lỗi cú pháp yêu cầu", nil)
	}

	if resp.StatusCode != http.StatusOK || !envelope.Success || envelope.Data == nil {
		slog.Error("location-service returned non-200 or failure", "status", resp.StatusCode, "body", string(bodyBytes))
		return nil, exception.NewServiceUnavailableError("location-service trả trạng thái không thành công", nil)
	}

	if envelope.Data.Candidates == nil {
		return []DriverCandidate{}, nil
	}

	return envelope.Data.Candidates, nil
}
