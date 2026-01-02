package handler

import (
	"context"
	"errors"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/sakashimaa/go-pet-project/pkg/mylogger"
	"github.com/sakashimaa/go-pet-project/pkg/utils"
	pb "github.com/sakashimaa/go-pet-project/proto/auth"
	"github.com/sony/gobreaker"
	"go.uber.org/zap"
)

type AuthHandler struct {
	client   pb.AuthServiceClient
	validate *validator.Validate
	cb       *gobreaker.CircuitBreaker
	logger   *zap.Logger
}

type RegisterInput struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=3"`
}

func NewAuthHandler(client pb.AuthServiceClient, logger *zap.Logger) *AuthHandler {
	settings := gobreaker.Settings{
		Name:        "AuthService",
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

	return &AuthHandler{
		client:   client,
		validate: validator.New(),
		cb:       gobreaker.NewCircuitBreaker(settings),
		logger:   logger,
	}
}

func (h *AuthHandler) GetMe(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.UserContext(), time.Second)
	defer cancel()

	userId, ok := c.Locals("userId").(int64)
	if !ok {
		mylogger.Info(
			ctx,
			h.logger,
			"user_id get failed",
		)

		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "userId parsing error"})
	}

	res, err := utils.ExecuteWithBreaker[*pb.UserInfoResponse](h.cb, func() (*pb.UserInfoResponse, error) {
		return h.client.GetUserInfo(ctx, &pb.UserInfoRequest{UserId: userId})
	})

	if err != nil {
		if errors.Is(err, gobreaker.ErrOpenState) {
			mylogger.Warn(ctx, h.logger, "Circuit breaker is open")

			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"error": "service temporarily unavailable",
			})
		}

		httpCode := utils.GRPCStatusToHTTP(err)

		mylogger.Warn(
			ctx,
			h.logger,
			"get me failed",
			zap.Int("http_code", httpCode),
			zap.Int64("user_id", userId),
			zap.Error(err),
		)

		return c.Status(httpCode).JSON(fiber.Map{"error": err.Error()})
	}

	mylogger.Info(
		ctx,
		h.logger,
		"get me succeeded",
		zap.Int64("user_id", userId),
		zap.String("email", res.Email),
	)

	return c.JSON(fiber.Map{
		"id":           userId,
		"email":        res.Email,
		"is_activated": res.IsActivated,
	})
}

func (h *AuthHandler) ResetPassword(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.UserContext(), time.Second)
	defer cancel()

	req := new(pb.ResetPasswordRequest)

	if err := c.BodyParser(req); err != nil {
		mylogger.Warn(
			ctx,
			h.logger,
			"body parsing error in reset password",
			zap.Error(err),
		)

		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Cannot parse JSON",
		})
	}

	token := c.Query("token")
	if token == "" {
		mylogger.Warn(
			ctx,
			h.logger,
			"token is invalid",
		)

		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid token",
		})
	}

	req.Token = token

	res, err := utils.ExecuteWithBreaker[*pb.ResetPasswordResponse](h.cb, func() (*pb.ResetPasswordResponse, error) {
		return h.client.ResetPassword(ctx, req)
	})

	if err != nil {
		if errors.Is(err, gobreaker.ErrOpenState) {
			mylogger.Warn(
				ctx,
				h.logger,
				"Circuit breaker is open",
				zap.String("method_name", "ResetPassword"),
			)
		}

		httpCode := utils.GRPCStatusToHTTP(err)

		mylogger.Warn(
			ctx,
			h.logger,
			"reset password failed",
			zap.Int("http_code", httpCode),
		)

		return c.Status(httpCode).JSON(fiber.Map{"error": err.Error()})
	}

	mylogger.Info(
		ctx,
		h.logger,
		"reset password succeeded",
		zap.String("token", req.Token),
	)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"success": res.Success})
}

func (h *AuthHandler) ForgotPassword(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.UserContext(), time.Second)
	defer cancel()

	req := new(pb.ForgotPasswordRequest)

	if err := c.BodyParser(req); err != nil {
		mylogger.Warn(
			ctx,
			h.logger,
			"body parsing error",
			zap.Error(err),
		)

		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Cannot parse JSON",
		})
	}

	if req.Email == "" {
		mylogger.Warn(
			ctx,
			h.logger,
			"email is invalid",
			zap.String("email", req.Email),
		)

		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "email is required",
		})
	}

	res, err := utils.ExecuteWithBreaker[*pb.ForgotPasswordResponse](h.cb, func() (*pb.ForgotPasswordResponse, error) {
		return h.client.ForgotPassword(ctx, req)
	})

	if err != nil {
		if errors.Is(err, gobreaker.ErrOpenState) {
			mylogger.Warn(
				ctx,
				h.logger,
				"Circuit breaker state is open",
				zap.String("method_name", "ForgotPassword"),
			)

			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"error": "service is temporarily unavailable",
			})
		}

		httpCode := utils.GRPCStatusToHTTP(err)

		mylogger.Warn(
			ctx,
			h.logger,
			"forgot password failed",
			zap.Error(err),
		)

		return c.Status(httpCode).JSON(fiber.Map{"error": err.Error()})
	}

	mylogger.Info(
		ctx,
		h.logger,
		"forgot password succeeded",
		zap.String("email", req.Email),
	)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": res.Success,
		"message": res.Message,
	})
}

func (h *AuthHandler) Activate(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.UserContext(), time.Second)
	defer cancel()

	req := new(pb.VerifyRequest)

	verifyToken := c.Query("token")
	if verifyToken == "" {
		mylogger.Warn(
			ctx,
			h.logger,
			"verify token is invalid",
			zap.String("verify_token", verifyToken),
		)

		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid token",
		})
	}

	req.Token = verifyToken

	res, err := utils.ExecuteWithBreaker[*pb.VerifyResponse](h.cb, func() (*pb.VerifyResponse, error) {
		return h.client.VerifyUser(ctx, req)
	})

	if err != nil {
		if errors.Is(err, gobreaker.ErrOpenState) {
			mylogger.Warn(
				ctx,
				h.logger,
				"Circuit breaker state open",
				zap.String("method_name", "Activate"),
			)

			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"error": "service temporarily unavailable",
			})
		}

		httpCode := utils.GRPCStatusToHTTP(err)

		mylogger.Warn(
			ctx,
			h.logger,
			"activate failed",
			zap.String("verify_token", verifyToken),
			zap.Error(err),
		)

		return c.Status(httpCode).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	mylogger.Info(
		ctx,
		h.logger,
		"activate succeeded",
		zap.Bool("success", res.Success),
	)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": res.Success,
	})
}

func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.UserContext(), time.Second)
	defer cancel()

	req := new(pb.LogoutRequest)

	if err := c.BodyParser(req); err != nil {
		mylogger.Warn(
			ctx,
			h.logger,
			"body parsing error",
			zap.Error(err),
		)

		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Cannot parse JSON",
		})
	}

	if req.RefreshToken == "" {
		mylogger.Warn(
			ctx,
			h.logger,
			"refresh token is invalid",
			zap.String("refresh_token", req.RefreshToken),
		)

		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "refresh token is required",
		})
	}

	res, err := utils.ExecuteWithBreaker[*pb.LogoutResponse](h.cb, func() (*pb.LogoutResponse, error) {
		return h.client.Logout(ctx, req)
	})

	if err != nil {
		if errors.Is(err, gobreaker.ErrOpenState) {
			mylogger.Warn(
				ctx,
				h.logger,
				"Circuit breaker state is open",
				zap.String("method_name", "Logout"),
			)

			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"error": "service is temporarily unavailable",
			})
		}

		httpCode := utils.GRPCStatusToHTTP(err)

		mylogger.Warn(
			ctx,
			h.logger,
			"logout failed",
			zap.String("refresh_token", req.RefreshToken),
		)

		return c.Status(httpCode).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	mylogger.Info(
		ctx,
		h.logger,
		"logout succeeded",
		zap.Bool("success", res.Success),
	)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": res.Success,
	})
}

func (h *AuthHandler) Refresh(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	req := new(pb.RefreshRequest)

	if err := c.BodyParser(req); err != nil {
		mylogger.Warn(
			ctx,
			h.logger,
			"body parsing error",
			zap.Error(err),
		)

		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Cannot parse JSON",
		})
	}

	if req.RefreshToken == "" {
		mylogger.Warn(
			ctx,
			h.logger,
			"refreshToken failed",
			zap.String("refresh_token", req.RefreshToken),
		)

		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "refresh token is required",
		})
	}

	res, err := utils.ExecuteWithBreaker[*pb.RefreshResponse](h.cb, func() (*pb.RefreshResponse, error) {
		return h.client.RefreshUser(ctx, req)
	})

	if err != nil {
		if errors.Is(err, gobreaker.ErrOpenState) {
			mylogger.Warn(
				ctx,
				h.logger,
				"Circuit breaker state is open",
				zap.String("method_name", "Refresh"),
			)

			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"error": "service is temporarily unavailable",
			})
		}

		httpCode := utils.GRPCStatusToHTTP(err)

		mylogger.Warn(
			ctx,
			h.logger,
			"refresh failed",
			zap.String("refresh_token", req.RefreshToken),
		)

		return c.Status(httpCode).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"refresh_token": res.RefreshToken,
		"access_token":  res.AccessToken,
	})
}

func (h *AuthHandler) Register(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.UserContext(), time.Second)
	defer cancel()

	input := new(RegisterInput)

	if err := c.BodyParser(&input); err != nil {
		mylogger.Warn(
			ctx,
			h.logger,
			"failed to parse body in register",
			zap.Error(err),
		)

		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "error parsing body",
		})
	}

	if err := h.validate.Struct(input); err != nil {
		mylogger.Warn(
			ctx,
			h.logger,
			"failed to parse input",
			zap.Error(err),
		)

		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": utils.FormatValidationError(err),
		})
	}

	res, err := utils.ExecuteWithBreaker[*pb.RegisterResponse](h.cb, func() (*pb.RegisterResponse, error) {
		req := pb.RegisterRequest{
			Email:    input.Email,
			Password: input.Password,
		}

		return h.client.Register(ctx, &req)
	})

	if err != nil {
		if errors.Is(err, gobreaker.ErrOpenState) {
			mylogger.Warn(
				ctx,
				h.logger,
				"Circuit breaker state is open",
				zap.String("method_name", "Register"),
			)

			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"error": "service temporarily unavailable",
			})
		}

		httpCode := utils.GRPCStatusToHTTP(err)

		mylogger.Warn(
			ctx,
			h.logger,
			"register failed",
			zap.Int("http_code", httpCode),
			zap.Error(err),
		)

		return c.Status(httpCode).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	mylogger.Info(
		ctx,
		h.logger,
		"register user succeeded",
		zap.Int64("created_id", res.Id),
	)

	return c.Status(fiber.StatusCreated).JSON(res)
}

func (h *AuthHandler) Login(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.UserContext(), time.Second)
	defer cancel()

	req := new(pb.LoginRequest)

	if err := c.BodyParser(&req); err != nil {
		mylogger.Warn(
			ctx,
			h.logger,
			"body parsing failed",
			zap.Error(err),
		)

		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Cannot parse JSON",
		})
	}

	if req.Email == "" || req.Password == "" {
		mylogger.Warn(
			ctx,
			h.logger,
			"email or password are invalid",
			zap.String("email", req.Email),
		)

		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Email and Password are required",
		})
	}

	res, err := utils.ExecuteWithBreaker[*pb.LoginResponse](h.cb, func() (*pb.LoginResponse, error) {
		return h.client.Login(ctx, req)
	})

	if err != nil {
		httpCode := utils.GRPCStatusToHTTP(err)

		mylogger.Warn(
			ctx,
			h.logger,
			"login failed",
			zap.Error(err),
		)

		return c.Status(httpCode).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(res)
}
