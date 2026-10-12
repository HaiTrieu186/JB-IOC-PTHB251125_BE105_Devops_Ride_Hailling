package controller

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"ws-gateway/internal/config"
	"ws-gateway/internal/service"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
)

const maxMessageSize = 4096

type rawGPSFrame struct {
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
}

type WSController struct {
	cfg               *config.Config
	hub               service.ConnectionHub
	locationForwarder service.LocationForwarder
}

func NewWSController(cfg *config.Config, hub service.ConnectionHub, locationForwarder service.LocationForwarder) *WSController {
	return &WSController{
		cfg:               cfg,
		hub:               hub,
		locationForwarder: locationForwarder,
	}
}

func (ctl *WSController) Health(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"status": "ok"})
}

func (ctl *WSController) HandleWS(c *websocket.Conn) {
	userID, _ := c.Locals("user_id").(string)
	role, _ := c.Locals("role").(string)

	s := service.NewSession(userID, role, c, ctl.cfg.ClientBufferSize, ctl.hub)
	if !ctl.hub.Register(s) {
		s.CloseWithCode(websocket.CloseTryAgainLater, "max connections exceeded")
		return
	}

	go ctl.writePump(s)
	ctl.readPump(s)
}

func (ctl *WSController) writePump(s *service.Session) {
	ticker := time.NewTicker(time.Duration(ctl.cfg.PingIntervalSeconds) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.Done():
			return
		case msg, ok := <-s.SendChan():
			if !ok {
				return
			}
			_ = s.Conn().SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := s.Conn().WriteMessage(websocket.TextMessage, msg); err != nil {
				s.CloseWithCode(websocket.CloseGoingAway, "write error")
				return
			}
		case <-ticker.C:
			_ = s.Conn().SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := s.Conn().WriteMessage(websocket.PingMessage, nil); err != nil {
				s.CloseWithCode(websocket.CloseGoingAway, "ping error")
				return
			}
		}
	}
}

func (ctl *WSController) readPump(s *service.Session) {
	defer func() {
		s.CloseWithCode(websocket.CloseGoingAway, "connection closed")
	}()

	pongWait := time.Duration(ctl.cfg.PongWaitSeconds) * time.Second
	s.Conn().SetReadLimit(maxMessageSize)
	_ = s.Conn().SetReadDeadline(time.Now().Add(pongWait))

	s.Conn().SetPongHandler(func(string) error {
		_ = s.Conn().SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		messageType, payload, err := s.Conn().ReadMessage()
		if err != nil {
			// Hết hạn hoặc ngắt kết nối
			break
		}

		// Gia hạn read deadline
		_ = s.Conn().SetReadDeadline(time.Now().Add(pongWait))

		// Chỉ DRIVER được gửi GPS, role khác gửi thì bỏ qua im lặng
		if s.Role != "DRIVER" {
			continue
		}

		if messageType != websocket.TextMessage && messageType != websocket.BinaryMessage {
			continue
		}

		var gps rawGPSFrame
		if err := json.Unmarshal(payload, &gps); err != nil {
			slog.Warn("invalid gps frame: not json", "user_id", s.UserID)
			continue
		}

		if gps.Latitude == nil || gps.Longitude == nil {
			slog.Warn("invalid gps frame: missing latitude or longitude", "user_id", s.UserID)
			continue
		}

		lat := *gps.Latitude
		lng := *gps.Longitude

		// Dải tọa độ: vĩ độ [-85.05112878, 85.05112878], kinh độ [-180, 180]
		if lat < -85.05112878 || lat > 85.05112878 || lng < -180.0 || lng > 180.0 {
			slog.Warn("invalid gps frame: coordinates out of range", "user_id", s.UserID, "latitude", lat, "longitude", lng)
			continue
		}

		ctl.locationForwarder.HandleGPS(context.Background(), s.UserID, lat, lng)
	}
}
