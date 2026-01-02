package handler

import (
	"context"
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/sakashimaa/go-pet-project/pkg/mylogger"
	"github.com/sakashimaa/go-pet-project/pkg/utils"
	pb "github.com/sakashimaa/go-pet-project/proto/order"
	"github.com/sony/gobreaker"
	"go.uber.org/zap"
)

type OrderHandler struct {
	client pb.OrderServiceClient
	logger *zap.Logger
	cb     *gobreaker.CircuitBreaker
}

func NewOrderHandler(client pb.OrderServiceClient, logger *zap.Logger) *OrderHandler {
	settings := gobreaker.Settings{
		Name:        "OrderService",
		MaxRequests: 3,
		Interval:    5 * time.Second,
		Timeout:     10 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 5 && failureRatio >= 0.6
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			logger.Warn(
				"Circuit breaker state changed",
				zap.String("name", name),
				zap.String("from", from.String()),
				zap.String("to", to.String()),
			)
		},
	}

	return &OrderHandler{
		client: client,
		logger: logger,
		cb:     gobreaker.NewCircuitBreaker(settings),
	}
}

func (h *OrderHandler) Create(c *fiber.Ctx) error {
	input := new(pb.CreateOrderRequest)

	if err := c.BodyParser(&input); err != nil {
		h.logger.Warn(
			"failed to parse body in create",
			zap.Error(err),
		)

		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "error parsing body",
		})
	}

	userId, ok := c.Locals("userId").(int64)
	if !ok {
		mylogger.Info(
			c.UserContext(),
			h.logger,
			"user_id get failed",
		)

		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "userId parsing error"})
	}

	result, err := h.cb.Execute(func() (interface{}, error) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		req := pb.CreateOrderRequest{
			UserId: userId,
			Items:  input.Items,
		}

		return h.client.CreateOrder(ctx, &req)
	})

	if err != nil {
		if errors.Is(err, gobreaker.ErrOpenState) {
			h.logger.Warn("Circuit breaker open")

			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"error": "service temporarily unavailable",
			})
		}

		httpCode := utils.GRPCStatusToHTTP(err)

		h.logger.Warn(
			"create order failed",
			zap.Int("http_code", httpCode),
			zap.Error(err),
		)

		return c.Status(httpCode).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	res, ok := result.(*pb.CreateOrderResponse)
	if !ok {
		h.logger.Warn("result cast error")

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "internal error",
		})
	}

	h.logger.Info(
		"create order succeeded",
		zap.Int64("created_id", res.OrderId),
	)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"order_id": res.OrderId,
		"status":   "success",
	})
}
