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

type UserClient interface {
	FilterOnlineDrivers(ctx context.Context, driverIDs []string) ([]string, error)
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

type filterOnlineResponse struct {
	Success bool `json:"success"`
	Data    struct {
		OnlineDriverIDs []string `json:"online_driver_ids"`
	} `json:"data"`
	Error any `json:"error"`
}

func (c *userClientImpl) FilterOnlineDrivers(ctx context.Context, driverIDs []string) ([]string, error) {
	reqCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	payload, err := json.Marshal(filterOnlineRequest{DriverIDs: driverIDs})
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/internal/v1/users/filter-online", c.baseURL)
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
		return nil, fmt.Errorf("user-service returned status %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var res filterOnlineResponse
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return nil, err
	}

	return res.Data.OnlineDriverIDs, nil
}
