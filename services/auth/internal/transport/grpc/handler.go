package grpc

import (
	"context"

	"github.com/sakashimaa/go-pet-project/auth/internal/service"
	"github.com/sakashimaa/go-pet-project/pkg/mylogger"
	pb "github.com/sakashimaa/go-pet-project/proto/auth"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AuthHandler struct {
	pb.UnimplementedAuthServiceServer
	service service.AuthService
	logger  *zap.Logger
}

func NewAuthHandler(service service.AuthService, logger *zap.Logger) *AuthHandler {
	return &AuthHandler{service: service, logger: logger}
}

func (h *AuthHandler) ResetPassword(ctx context.Context, req *pb.ResetPasswordRequest) (*pb.ResetPasswordResponse, error) {
	res, err := h.service.ResetPassword(ctx, req)
	if err != nil {
		code := mapErrorCode(err)

		mylogger.Warn(
			ctx,
			h.logger,
			"Reset password failed",
			zap.Error(err),
			zap.String("status_code", err.Error()),
		)

		return nil, status.Error(code, err.Error())
	}

	mylogger.Info(
		ctx,
		h.logger,
		"Reset password succeeded",
		zap.Bool("success", res.Success),
	)

	return res, nil
}

func (h *AuthHandler) ForgotPassword(ctx context.Context, req *pb.ForgotPasswordRequest) (*pb.ForgotPasswordResponse, error) {
	res, err := h.service.ForgotPassword(ctx, req)
	if err != nil {
		code := mapErrorCode(err)

		mylogger.Warn(
			ctx,
			h.logger,
			"Forgot password failed",
			zap.String("status_code", err.Error()),
			zap.Error(err),
		)

		return nil, status.Error(code, err.Error())
	}

	mylogger.Info(
		ctx,
		h.logger,
		"Forgot password succeeded",
		zap.String("email", req.Email),
	)

	return res, nil
}

func (h *AuthHandler) VerifyUser(ctx context.Context, req *pb.VerifyRequest) (*pb.VerifyResponse, error) {
	res, err := h.service.Verify(ctx, req)
	if err != nil {
		code := mapErrorCode(err)

		mylogger.Warn(
			ctx,
			h.logger,
			"Verify User failed",
			zap.String("status_code", err.Error()),
			zap.Error(err),
		)

		return nil, status.Error(code, err.Error())
	}

	return &pb.VerifyResponse{
		Success: res.Success,
	}, nil
}

func (h *AuthHandler) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	user, err := h.service.Register(ctx, req.Email, req.Password)
	if err != nil {
		code := mapErrorCode(err)

		mylogger.Warn(
			ctx,
			h.logger,
			"Register failed",
			zap.String("status_code", err.Error()),
			zap.Error(err),
		)

		return nil, status.Error(code, err.Error())
	}

	return &pb.RegisterResponse{
		Id:              user.ID,
		Email:           req.Email,
		ActivationToken: user.ActivationToken,
		CreatedAt:       user.CreatedAt.String(),
		UpdatedAt:       user.UpdatedAt.String(),
	}, nil
}

func (h *AuthHandler) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	if req.Email == "" || req.Password == "" {
		mylogger.Info(
			ctx,
			h.logger,
			"Email and Password are invalid",
			zap.String("method_name", "Login"),
		)

		return nil, status.Error(codes.InvalidArgument, "email and password are required")
	}

	access, refresh, err := h.service.Login(ctx, req.Email, req.Password)
	if err != nil {
		code := mapErrorCode(err)

		mylogger.Warn(
			ctx,
			h.logger,
			"Login failed",
			zap.String("email", req.Email),
			zap.Error(err),
		)

		return nil, status.Error(code, err.Error())
	}

	return &pb.LoginResponse{
		AccessToken:  access,
		RefreshToken: refresh,
	}, nil
}

func (h *AuthHandler) ValidateUser(ctx context.Context, req *pb.ValidateRequest) (*pb.ValidateResponse, error) {
	if req.Token == "" {
		mylogger.Info(
			ctx,
			h.logger,
			"No token provided",
			zap.String("token", req.Token),
		)

		return nil, status.Error(codes.InvalidArgument, "no token provided")
	}

	res, err := h.service.Validate(ctx, req.Token)
	if err != nil {
		code := mapErrorCode(err)

		mylogger.Warn(
			ctx,
			h.logger,
			"Validate User failed",
			zap.String("error_code", err.Error()),
			zap.Error(err),
		)

		return nil, status.Error(code, err.Error())
	}

	return &pb.ValidateResponse{
		UserId:      res.UserId,
		IsActivated: res.IsActivated,
	}, nil
}

func (h *AuthHandler) GetUserInfo(ctx context.Context, req *pb.UserInfoRequest) (*pb.UserInfoResponse, error) {
	res, err := h.service.GetUserInfo(ctx, req.UserId)

	if err != nil {
		code := mapErrorCode(err)

		mylogger.Warn(
			ctx,
			h.logger,
			"GetUserInfo failed",
			zap.String("status_code", err.Error()),
			zap.Error(err),
		)

		return nil, status.Error(code, err.Error())
	}

	return &pb.UserInfoResponse{
		Email:       res.Email,
		IsActivated: res.IsActivated,
	}, nil
}

func (h *AuthHandler) RefreshUser(ctx context.Context, req *pb.RefreshRequest) (*pb.RefreshResponse, error) {
	res, err := h.service.Refresh(ctx, req)
	if err != nil {
		code := mapErrorCode(err)

		mylogger.Warn(
			ctx,
			h.logger,
			"Refresh failed",
			zap.String("status_code", err.Error()),
			zap.Error(err),
		)

		return nil, status.Error(code, err.Error())
	}

	return &pb.RefreshResponse{
		RefreshToken: res.RefreshToken,
		AccessToken:  res.AccessToken,
	}, nil
}

func (h *AuthHandler) Logout(ctx context.Context, req *pb.LogoutRequest) (*pb.LogoutResponse, error) {
	res, err := h.service.Logout(ctx, req)
	if err != nil {
		code := mapErrorCode(err)

		mylogger.Warn(
			ctx,
			h.logger,
			"Logout failed",
			zap.String("status_code", err.Error()),
			zap.Error(err),
		)

		return nil, status.Error(code, err.Error())
	}

	return &pb.LogoutResponse{
		Success: res.Success,
	}, nil
}
