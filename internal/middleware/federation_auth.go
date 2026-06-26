package middleware

import (
	"github.com/corvych/nit/internal/repository"
	"github.com/gofiber/fiber/v3"
)

func NewFederationAuth(repo repository.TrustedNodeRepository) fiber.Handler {
	return func(c fiber.Ctx) error {
		key := c.Get("X-Federation-Key")
		if key == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Missing X-Federation-Key header",
			})
		}

		node, err := repo.GetByAPIKey(c, key)
		if err != nil || node == nil || node.Status != "active" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid or inactive federation key",
			})
		}

		c.Locals("nodeDomain", node.Domain)
		return c.Next()
	}
}
