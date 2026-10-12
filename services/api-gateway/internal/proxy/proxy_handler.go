package proxy

import (
	"fmt"
	"time"

	"api-gateway/internal/config"
	"api-gateway/internal/exception"

	"github.com/gofiber/fiber/v2"
	fiberProxy "github.com/gofiber/fiber/v2/middleware/proxy"
)

type ProxyHandler struct {
	cfg *config.Config
}

func NewProxyHandler(cfg *config.Config) *ProxyHandler {
	return &ProxyHandler{cfg: cfg}
}

func (h *ProxyHandler) Forward(targetBaseURL string, isUserService bool) fiber.Handler {
	timeout := time.Duration(h.cfg.ProxyTimeoutMS) * time.Millisecond
	return func(c *fiber.Ctx) error {
		if !isUserService {
			c.Request().Header.Del(fiber.HeaderAuthorization)
		}

		targetURL := fmt.Sprintf("%s%s", targetBaseURL, c.OriginalURL())

		if err := fiberProxy.DoTimeout(c, targetURL, timeout); err != nil {
			return exception.NewServiceUnavailableError("Dịch vụ đích không phản hồi hoặc quá thời gian chờ", err)
		}

		return nil
	}
}
