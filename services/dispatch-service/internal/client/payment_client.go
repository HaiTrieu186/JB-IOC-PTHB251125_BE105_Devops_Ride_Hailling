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

	"github.com/google/uuid"

	"dispatch-service/internal/exception"
)

type CheckBalanceResult struct {
	Sufficient bool  `json:"sufficient"`
	Balance    int64 `json:"balance"`
}

type PaymentClient interface {
	CheckBalance(ctx context.Context, userID uuid.UUID, requiredAmount int64) (*CheckBalanceResult, error)
}

type paymentClientImpl struct {
	baseURL string
	timeout time.Duration
}

func NewPaymentClient(baseURL string, timeoutMs int) PaymentClient {
	return &paymentClientImpl{
		baseURL: baseURL,
		timeout: time.Duration(timeoutMs) * time.Millisecond,
	}
}

type checkBalanceRequest struct {
	UserID         string `json:"user_id"`
	RequiredAmount int64  `json:"required_amount"`
}

type paymentEnvelope struct {
	Success bool                `json:"success"`
	Data    *CheckBalanceResult `json:"data"`
	Error   *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (c *paymentClientImpl) CheckBalance(ctx context.Context, userID uuid.UUID, requiredAmount int64) (*CheckBalanceResult, error) {
	reqCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	payload, err := json.Marshal(checkBalanceRequest{
		UserID:         userID.String(),
		RequiredAmount: requiredAmount,
	})
	if err != nil {
		return nil, exception.NewServiceUnavailableError("Lỗi đóng gói request tới payment-service", err)
	}

	url := fmt.Sprintf("%s/internal/v1/wallets/check-balance", c.baseURL)
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, exception.NewServiceUnavailableError("Lỗi khởi tạo request tới payment-service", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := SharedHTTPClient.Do(req)
	if err != nil {
		slog.Error("payment-service network error / timeout", "error", err)
		return nil, exception.NewServiceUnavailableError("payment-service không phản hồi hoặc timeout", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, exception.NewServiceUnavailableError("Lỗi đọc body từ payment-service", err)
	}

	var envelope paymentEnvelope
	if err := json.Unmarshal(bodyBytes, &envelope); err != nil {
		return nil, exception.NewServiceUnavailableError("Lỗi giải mã response từ payment-service", err)
	}

	if resp.StatusCode == http.StatusBadRequest {
		slog.Error("payment-service returned 400 (invalid request from dispatch)", "body", string(bodyBytes))
		return nil, exception.NewServiceUnavailableError("payment-service trả lỗi cú pháp yêu cầu", nil)
	}

	if resp.StatusCode != http.StatusOK || !envelope.Success || envelope.Data == nil {
		slog.Error("payment-service returned non-200 or failure", "status", resp.StatusCode, "body", string(bodyBytes))
		return nil, exception.NewServiceUnavailableError("payment-service trả trạng thái không thành công", nil)
	}

	return envelope.Data, nil
}
