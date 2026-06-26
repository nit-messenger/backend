package middleware

import (
	"fmt"
	"strings"

	"github.com/corvych/nit/internal/config"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/gofiber/fiber/v3"
)

func NewAuthMiddleware(cfg *config.Config) fiber.Handler {
	return func(c fiber.Ctx) error {
		// 1. Get token from Authorization header or Query param (for WS connections)
		var tokenStr string
		authHeader := c.Get("Authorization")
		if authHeader != "" {
			parts := strings.Split(authHeader, " ")
			if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
				tokenStr = parts[1]
			}
		}

		if tokenStr == "" {
			// Check query param (e.g. /ws?token=XYZ)
			tokenStr = c.Query("token")
		}

		if tokenStr == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Missing or malformed authorization token",
			})
		}

		// 2. Parse and validate the token
		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return []byte(cfg.JWTSecret), nil
		})

		if err != nil || !token.Valid {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid or expired authorization token",
			})
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid token claims",
			})
		}

		subStr, err := claims.GetSubject()
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Subject claim missing in token",
			})
		}

		userID, err := uuid.Parse(subStr)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid user ID format in token subject",
			})
		}

		// 3. Set the userID in c.Locals for down-stream handlers
		c.Locals("userID", userID)

		return c.Next()
	}
}

// Helper to extract authenticated user ID in handlers
func GetUserID(c fiber.Ctx) (uuid.UUID, error) {
	val := c.Locals("userID")
	if val == nil {
		return uuid.Nil, fiber.NewError(fiber.StatusUnauthorized, "User context not found")
	}
	userID, ok := val.(uuid.UUID)
	if !ok {
		return uuid.Nil, fiber.NewError(fiber.StatusUnauthorized, "Invalid user ID type in context")
	}
	return userID, nil
}
