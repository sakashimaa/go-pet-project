package handler

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/sakashimaa/go-pet-project/pkg/mylogger"
	"github.com/sakashimaa/go-pet-project/pkg/utils"
	pb "github.com/sakashimaa/go-pet-project/proto/product"
	"github.com/sony/gobreaker"
	"go.uber.org/zap"
)

type ProductHandler struct {
	client   pb.ProductServiceClient
	validate *validator.Validate
	logger   *zap.Logger
	cb       *gobreaker.CircuitBreaker
}

func NewProductHandler(client pb.ProductServiceClient, logger *zap.Logger) *ProductHandler {
	settings := gobreaker.Settings{
		Name:        "ProductService",
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

	return &ProductHandler{
		client:   client,
		validate: validator.New(),
		logger:   logger,
		cb:       gobreaker.NewCircuitBreaker(settings),
	}
}

type CreateProductInput struct {
	Name          string `json:"name" validate:"required,min=3,max=100"`
	Description   string `json:"description" validate:"max=1000"`
	Price         int64  `json:"price" validate:"required,gt=0"`
	StockQuantity int64  `json:"stock_quantity" validate:"gte=0"`
	Category      string `json:"category" validate:"required"`
	ImageUrl      string `json:"image_url" validate:"omitempty,url"`
}

func (h *ProductHandler) DeleteProduct(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.UserContext(), time.Second)
	defer cancel()

	idStr := c.Params("id")
	id, err := strconv.Atoi(idStr)

	if err != nil {
		mylogger.Warn(
			ctx,
			h.logger,
			"invalid product id",
			zap.String("id", idStr),
		)

		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Id is invalid",
		})
	}

	mylogger.Info(
		ctx,
		h.logger,
		"delete product request",
		zap.Int("product_id", id),
	)

	req := new(pb.DeleteProductRequest)
	req.Id = int64(id)

	result, err := h.cb.Execute(func() (interface{}, error) {
		return h.client.DeleteProduct(ctx, req)
	})

	if err != nil {
		if errors.Is(err, gobreaker.ErrOpenState) {
			mylogger.Warn(ctx, h.logger, "Circuit breaker open")

			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"error": "Service temporarily unavailable",
			})
		}

		httpStatus := utils.GRPCStatusToHTTP(err)

		mylogger.Warn(
			ctx,
			h.logger,
			"delete product failed",
			zap.Int("product_id", id),
			zap.Int("http_status", httpStatus),
			zap.Error(err),
		)

		return c.Status(httpStatus).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	res, ok := result.(*pb.DeleteProductResponse)
	if !ok {
		mylogger.Warn(ctx, h.logger, "result cast failed")

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "result cast failed",
		})
	}

	mylogger.Info(
		ctx,
		h.logger,
		"product deleted successfully",
		zap.Int("product_id", id),
	)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": res.Success,
	})
}

func (h *ProductHandler) DecreaseStock(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.UserContext(), time.Second)
	defer cancel()

	req := new(pb.DecreaseStockRequest)

	if err := c.BodyParser(req); err != nil {
		mylogger.Warn(
			ctx,
			h.logger,
			"body parsing failed",
			zap.Error(err),
		)

		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	idStr := c.Params("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		mylogger.Warn(
			ctx,
			h.logger,
			"invalid product id",
			zap.String("id", idStr),
		)

		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid product id",
		})
	}

	req.ProductId = int64(id)

	if req.Quantity == 0 {
		mylogger.Warn(
			ctx,
			h.logger,
			"quantity is invalid",
			zap.Int("product_id", id),
			zap.Int64("quantity", req.Quantity),
		)

		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "quantity is invalid",
		})
	}

	result, err := h.cb.Execute(func() (interface{}, error) {
		return h.client.DecreaseStock(ctx, req)
	})

	if err != nil {
		if errors.Is(err, gobreaker.ErrOpenState) {
			mylogger.Warn(ctx, h.logger, "Circuit breaker state open")
		}

		httpCode := utils.GRPCStatusToHTTP(err)

		mylogger.Warn(
			ctx,
			h.logger,
			"decrease stock failed",
			zap.Int("product_id", id),
			zap.Int("http_status", httpCode),
			zap.Error(err),
		)

		return c.Status(httpCode).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	res, ok := result.(*pb.DecreaseStockResponse)

	if !ok {
		mylogger.Warn(ctx, h.logger, "result cast failed")

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "result cast failed",
		})
	}

	mylogger.Info(
		ctx,
		h.logger,
		"decreased stock successfully",
		zap.Int("product_id", id),
	)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": res.Success,
		"message": res.Message,
	})
}

func (h *ProductHandler) ListProducts(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.UserContext(), time.Second)
	defer cancel()

	offsetStr := c.Query("offset")
	offset, err := strconv.Atoi(offsetStr)

	if err != nil {
		mylogger.Warn(
			ctx,
			h.logger,
			"offset is invalid",
			zap.String("offset", offsetStr),
		)

		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "offset is invalid",
		})
	}

	limitStr := c.Query("limit")
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		mylogger.Warn(
			ctx,
			h.logger,
			"limit is invalid",
			zap.String("limit", limitStr),
		)

		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "limit is invalid",
		})
	}

	search := c.Query("search")

	body, err := h.cb.Execute(func() (interface{}, error) {
		req := pb.ListProductsRequest{
			Offset: int64(offset),
			Limit:  int64(limit),
			Search: search,
		}

		return h.client.ListProducts(ctx, &req)
	})

	if err != nil {
		if errors.Is(err, gobreaker.ErrOpenState) {
			mylogger.Error(ctx, h.logger, "Circuit breaker is open, request blocked")

			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"error": "Product service is currently unavailable",
			})
		}

		httpCode := utils.GRPCStatusToHTTP(err)

		mylogger.Warn(
			ctx,
			h.logger,
			"list products failed",
			zap.Int("http_code", httpCode),
			zap.Error(err),
		)

		return c.Status(httpCode).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	res, ok := body.(*pb.ListProductsResponse)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal type error"})
	}

	mylogger.Info(
		ctx,
		h.logger,
		"list products succeeded",
		zap.Int("offset", offset),
		zap.Int("limit", limit),
		zap.String("search", search),
		zap.Int64("total", res.TotalCount),
	)

	return c.Status(fiber.StatusOK).JSON(res)
}

func (h *ProductHandler) FindByID(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.UserContext(), time.Second)
	defer cancel()

	idStr := c.Params("id")
	if idStr == "" {
		mylogger.Warn(
			ctx,
			h.logger,
			"id is invalid",
			zap.String("id", idStr),
		)

		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "id is required",
		})
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		mylogger.Warn(
			ctx,
			h.logger,
			"id is invalid",
			zap.String("id", idStr),
		)

		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid id",
		})
	}

	result, err := h.cb.Execute(func() (interface{}, error) {
		req := pb.GetProductRequest{
			Id: int64(id),
		}

		return h.client.GetProduct(ctx, &req)
	})

	if err != nil {
		if errors.Is(err, gobreaker.ErrOpenState) {
			mylogger.Warn(ctx, h.logger, "Circuit breaker open", zap.Int("product_id", id))

			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"error": "Service temporarily unavailable",
			})
		}

		httpCode := utils.GRPCStatusToHTTP(err)

		mylogger.Warn(
			ctx,
			h.logger,
			"find by id failed",
			zap.Int("id", id),
			zap.Int("http_code", httpCode),
			zap.Error(err),
		)

		return c.Status(httpCode).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	res, ok := result.(*pb.GetProductResponse)
	if !ok {
		mylogger.Error(
			ctx, h.logger, "failed to cast response", zap.Int("product_id", id),
		)

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "internal error",
		})
	}

	h.logger.Info(
		"find by id succeeded",
		zap.Int("product_id", id),
	)

	return c.Status(fiber.StatusOK).JSON(res)
}

func (h *ProductHandler) Create(c *fiber.Ctx) error {
	input := new(CreateProductInput)

	if err := c.BodyParser(&input); err != nil {
		h.logger.Warn(
			"failed to parse body in create",
			zap.Error(err),
		)

		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "error parsing body",
		})
	}

	if err := h.validate.Struct(input); err != nil {
		h.logger.Warn(
			"failed to parse input",
			zap.Error(err),
		)

		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": utils.FormatValidationError(err),
		})
	}

	result, err := h.cb.Execute(func() (interface{}, error) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		req := pb.CreateProductRequest{
			Name:          input.Name,
			Description:   input.Description,
			Price:         input.Price,
			StockQuantity: input.StockQuantity,
			Category:      input.Category,
		}

		return h.client.CreateProduct(ctx, &req)
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
			"create product failed",
			zap.Int("http_code", httpCode),
			zap.Error(err),
		)

		return c.Status(httpCode).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	res, ok := result.(*pb.CreateProductResponse)
	if !ok {
		h.logger.Warn("result cast error")

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "internal error",
		})
	}

	h.logger.Info(
		"create product succeeded",
		zap.Int64("created_id", res.Id),
	)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"id":     res.Id,
		"status": "success",
	})
}
