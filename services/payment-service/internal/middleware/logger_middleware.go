package middleware

import (
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
)

func LoggerMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		latency := time.Since(start).Milliseconds()

		slog.Info("http request",
			"method", c.Method(),
			"path", c.OriginalURL(),
			"status", c.Response().StatusCode(),
			"latency_ms", latency,
			"ip", c.IP(),
		)

		return err
	}
}
