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

type EstimateResult struct {
	Fare            int64   `json:"fare"`
	DistanceM       int     `json:"distance_m"`
	ETASeconds      int     `json:"eta_seconds"`
	SurgeMultiplier float64 `json:"surge_multiplier"`
}

type PricingClient interface {
	Estimate(ctx context.Context, pickupLat, pickupLng, dropoffLat, dropoffLng float64) (*EstimateResult, error)
}

type pricingClientImpl struct {
	baseURL string
	timeout time.Duration
}

func NewPricingClient(baseURL string, timeoutMs int) PricingClient {
	return &pricingClientImpl{
		baseURL: baseURL,
		timeout: time.Duration(timeoutMs) * time.Millisecond,
	}
}

type pricingEstimateRequest struct {
	PickupLat  float64 `json:"pickup_lat"`
	PickupLng  float64 `json:"pickup_lng"`
	DropoffLat float64 `json:"dropoff_lat"`
	DropoffLng float64 `json:"dropoff_lng"`
}

type pricingEnvelope struct {
	Success bool            `json:"success"`
	Data    *EstimateResult `json:"data"`
	Error   *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (c *pricingClientImpl) Estimate(ctx context.Context, pickupLat, pickupLng, dropoffLat, dropoffLng float64) (*EstimateResult, error) {
	reqCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	payload, err := json.Marshal(pricingEstimateRequest{
		PickupLat:  pickupLat,
		PickupLng:  pickupLng,
		DropoffLat: dropoffLat,
		DropoffLng: dropoffLng,
	})
	if err != nil {
		return nil, exception.NewServiceUnavailableError("Lỗi đóng gói request tới pricing-service", err)
	}

	url := fmt.Sprintf("%s/internal/v1/pricing/estimate", c.baseURL)
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, exception.NewServiceUnavailableError("Lỗi khởi tạo request tới pricing-service", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := SharedHTTPClient.Do(req)
	if err != nil {
		slog.Error("pricing-service network error / timeout", "error", err)
		return nil, exception.NewServiceUnavailableError("pricing-service không phản hồi hoặc timeout", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, exception.NewServiceUnavailableError("Lỗi đọc body từ pricing-service", err)
	}

	var envelope pricingEnvelope
	if err := json.Unmarshal(bodyBytes, &envelope); err != nil {
		return nil, exception.NewServiceUnavailableError("Lỗi giải mã response từ pricing-service", err)
	}

	// Ánh xạ lỗi theo Mục 4 LLD-00: pricing trả 400 VALIDATION_ERROR -> trả lại VALIDATION_ERROR (400)
	if resp.StatusCode == http.StatusBadRequest {
		msg := "Dữ liệu ước tính cước không hợp lệ"
		if envelope.Error != nil && envelope.Error.Message != "" {
			msg = envelope.Error.Message
		}
		return nil, exception.NewValidationError(msg)
	}

	if resp.StatusCode != http.StatusOK || !envelope.Success || envelope.Data == nil {
		slog.Error("pricing-service returned non-200 or failure", "status", resp.StatusCode, "body", string(bodyBytes))
		return nil, exception.NewServiceUnavailableError("pricing-service trả lỗi hoặc trạng thái không thành công", nil)
	}

	return envelope.Data, nil
}
