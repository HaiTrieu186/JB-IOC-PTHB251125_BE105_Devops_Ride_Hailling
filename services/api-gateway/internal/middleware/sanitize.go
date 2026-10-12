package middleware

import (
	"strings"

	"api-gateway/internal/exception"

	"github.com/gofiber/fiber/v2"
)

func Sanitize() fiber.Handler {
	return func(c *fiber.Ctx) error {
		var toDelete []string
		c.Request().Header.VisitAll(func(key, value []byte) {
			k := string(key)
			if strings.HasPrefix(strings.ToLower(k), "x-user-") {
				toDelete = append(toDelete, k)
			}
		})
		for _, k := range toDelete {
			c.Request().Header.Del(k)
		}

		if strings.HasPrefix(c.Path(), "/internal/") {
			return exception.NewNotFoundError("Đường dẫn không tồn tại")
		}

		return c.Next()
	}
}
