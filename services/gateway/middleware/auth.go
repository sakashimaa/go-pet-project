package middleware

import (
	"context"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	pb "github.com/sakashimaa/go-pet-project/proto/auth"
)

func NewAuthMiddleware(authClient pb.AuthServiceClient) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized: missed header"})
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized: Invalid header format"})
		}
		token := parts[1]

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		res, err := authClient.ValidateUser(ctx, &pb.ValidateRequest{Token: token})
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized: Invalid token"})
		}

		c.Locals("userId", res.UserId)
		c.Locals("isActivated", res.IsActivated)
		return c.Next()
	}
}

func NewIsActivatedMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		val := c.Locals("userId")
		userId, ok := val.(int64)
		if !ok || userId == 0 {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized: missed user"})
		}

		val = c.Locals("isActivated")
		isActivated, ok := val.(bool)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Internal error: auth flow violation"})
		}

		if !isActivated {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Account not activated",
				"code":  "EMAIL_NOT_VERIFIED",
			})
		}

		return c.Next()
	}
}
