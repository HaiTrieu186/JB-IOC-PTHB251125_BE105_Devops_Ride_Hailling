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

type UserClient interface {
	FilterOnline(ctx context.Context, driverIDs []string) ([]string, error)
	UpdateDriverStatus(ctx context.Context, driverID string, fromStatus, toStatus string) error
	UpdateDriverStatusWithRetry(driverID string, fromStatus, toStatus string) error
}

type userClientImpl struct {
	baseURL string
	timeout time.Duration
}

func NewUserClient(baseURL string, timeoutMs int) UserClient {
	return &userClientImpl{
		baseURL: baseURL,
		timeout: time.Duration(timeoutMs) * time.Millisecond,
	}
}

type filterOnlineRequest struct {
	DriverIDs []string `json:"driver_ids"`
}

type filterOnlineEnvelope struct {
	Success bool `json:"success"`
	Data    *struct {
		OnlineDriverIDs []string `json:"online_driver_ids"`
	} `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (c *userClientImpl) FilterOnline(ctx context.Context, driverIDs []string) ([]string, error) {
	reqCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Đảm bảo không nil để tránh JSON null gây 400 (Quyết định D)
	ids := make([]string, len(driverIDs))
	copy(ids, driverIDs)

	payload, err := json.Marshal(filterOnlineRequest{DriverIDs: ids})
	if err != nil {
		return nil, exception.NewServiceUnavailableError("Lỗi đóng gói request tới user-service", err)
	}

	url := fmt.Sprintf("%s/internal/v1/users/filter-online", c.baseURL)
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, exception.NewServiceUnavailableError("Lỗi khởi tạo request tới user-service", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := SharedHTTPClient.Do(req)
	if err != nil {
		slog.Error("user-service filter-online network error / timeout", "error", err)
		return nil, exception.NewServiceUnavailableError("user-service không phản hồi hoặc timeout", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, exception.NewServiceUnavailableError("Lỗi đọc body từ user-service", err)
	}

	var envelope filterOnlineEnvelope
	if err := json.Unmarshal(bodyBytes, &envelope); err != nil {
		return nil, exception.NewServiceUnavailableError("Lỗi giải mã response từ user-service", err)
	}

	if resp.StatusCode == http.StatusBadRequest {
		slog.Error("user-service filter-online returned 400 (dispatch sent invalid payload)", "body", string(bodyBytes))
		return nil, exception.NewServiceUnavailableError("user-service trả lỗi cú pháp yêu cầu", nil)
	}

	if resp.StatusCode != http.StatusOK || !envelope.Success || envelope.Data == nil {
		slog.Error("user-service filter-online returned non-200 or failure", "status", resp.StatusCode, "body", string(bodyBytes))
		return nil, exception.NewServiceUnavailableError("user-service trả trạng thái không thành công", nil)
	}

	if envelope.Data.OnlineDriverIDs == nil {
		return []string{}, nil
	}

	return envelope.Data.OnlineDriverIDs, nil
}

type updateStatusRequest struct {
	FromStatus string `json:"from_status"`
	ToStatus   string `json:"to_status"`
}

type updateStatusEnvelope struct {
	Success bool `json:"success"`
	Data    *struct {
		DriverID string `json:"driver_id"`
		Status   string `json:"status"`
	} `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (c *userClientImpl) UpdateDriverStatus(ctx context.Context, driverID string, fromStatus, toStatus string) error {
	reqCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	payload, err := json.Marshal(updateStatusRequest{
		FromStatus: fromStatus,
		ToStatus:   toStatus,
	})
	if err != nil {
		return exception.NewServiceUnavailableError("Lỗi đóng gói request đổi trạng thái tài xế", err)
	}

	url := fmt.Sprintf("%s/internal/v1/drivers/%s/status", c.baseURL, driverID)
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return exception.NewServiceUnavailableError("Lỗi khởi tạo request đổi trạng thái tài xế", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := SharedHTTPClient.Do(req)
	if err != nil {
		slog.Error("user-service update status network error / timeout", "driver_id", driverID, "error", err)
		return exception.NewServiceUnavailableError("user-service không phản hồi hoặc timeout", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return exception.NewServiceUnavailableError("Lỗi đọc body từ user-service", err)
	}

	var envelope updateStatusEnvelope
	if err := json.Unmarshal(bodyBytes, &envelope); err != nil {
		return exception.NewServiceUnavailableError("Lỗi giải mã response từ user-service", err)
	}

	// Quyết định D: Đọc error.code trong body khi HTTP 400
	if resp.StatusCode == http.StatusBadRequest {
		if envelope.Error != nil && envelope.Error.Code == exception.ErrCodeDriverNotAvailable {
			return exception.NewDriverNotAvailableError("Tài xế không ở trạng thái sẵn sàng để nhận chuyến")
		}
		slog.Error("user-service update status returned 400 (validation error from dispatch)", "driver_id", driverID, "body", string(bodyBytes))
		return exception.NewServiceUnavailableError("user-service báo lỗi cú pháp yêu cầu", nil)
	}

	if resp.StatusCode != http.StatusOK || !envelope.Success {
		slog.Error("user-service update status returned non-200 or failure", "status", resp.StatusCode, "body", string(bodyBytes))
		return exception.NewServiceUnavailableError("user-service đổi trạng thái thất bại", nil)
	}

	return nil
}

// UpdateDriverStatusWithRetry thực hiện đổi trạng thái tài xế (thường là BUSY -> ONLINE)
// với retry tối đa 3 lần, mỗi lần có timeout riêng 500 ms (Quyết định E)
// CHỈ retry khi lỗi mạng hoặc timeout. KHÔNG retry khi nhận DRIVER_NOT_AVAILABLE.
func (c *userClientImpl) UpdateDriverStatusWithRetry(driverID string, fromStatus, toStatus string) error {
	const maxAttempts = 3
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		err := c.UpdateDriverStatus(ctx, driverID, fromStatus, toStatus)
		cancel()

		if err == nil {
			if attempt > 1 {
				slog.Info("UpdateDriverStatus succeeded on retry", "driver_id", driverID, "attempt", attempt)
			}
			return nil
		}

		lastErr = err

		// Nếu là AppError có code DRIVER_NOT_AVAILABLE
		if ae, ok := err.(*exception.AppError); ok && ae.Code == exception.ErrCodeDriverNotAvailable {
			slog.Warn("driver not available, skipping retry", "driver_id", driverID, "attempt", attempt)
			return err
		}

		slog.Warn("retry failed for driver status update", "driver_id", driverID, "attempt", attempt, "error", err)
		if attempt < maxAttempts {
			time.Sleep(50 * time.Millisecond)
		}
	}

	slog.Error("failed all retries for driver status update", "driver_id", driverID, "attempts", maxAttempts, "error", lastErr)
	return lastErr
}
