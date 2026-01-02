package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sakashimaa/go-pet-project/auth/internal/domain"
	"github.com/sakashimaa/go-pet-project/auth/internal/repository"
	"github.com/sakashimaa/go-pet-project/auth/pkg/utils"
	"github.com/sakashimaa/go-pet-project/auth/pkg/validator"
	"github.com/sakashimaa/go-pet-project/pkg/mylogger"
	outboxDomain "github.com/sakashimaa/go-pet-project/pkg/outbox/domain"
	"github.com/sakashimaa/go-pet-project/pkg/outbox/worker"
	pb "github.com/sakashimaa/go-pet-project/proto/auth"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type AuthService interface {
	GetUserInfo(ctx context.Context, id int64) (*domain.User, error)
	Register(ctx context.Context, email, password string) (*domain.User, error)
	Login(ctx context.Context, email, password string) (string, string, error)
	Validate(ctx context.Context, token string) (*pb.ValidateResponse, error)
	Refresh(ctx context.Context, request *pb.RefreshRequest) (*pb.RefreshResponse, error)
	Logout(ctx context.Context, request *pb.LogoutRequest) (*pb.LogoutResponse, error)
	Verify(ctx context.Context, request *pb.VerifyRequest) (*pb.VerifyResponse, error)
	ForgotPassword(ctx context.Context, request *pb.ForgotPasswordRequest) (*pb.ForgotPasswordResponse, error)
	ResetPassword(ctx context.Context, request *pb.ResetPasswordRequest) (*pb.ResetPasswordResponse, error)
}

type authService struct {
	userRepo      repository.UserRepository
	outboxRepo    worker.OutboxRepository
	kafkaProducer EventProducer
	logger        *zap.Logger
	pool          *pgxpool.Pool
	validator     validator.Validator
}

type EventProducer interface {
	ProduceMessage(ctx context.Context, topic string, message interface{}) error
}

func NewAuthService(
	userRepo repository.UserRepository,
	outboxRepo worker.OutboxRepository,
	kafkaProducer EventProducer,
	logger *zap.Logger,
	pool *pgxpool.Pool,
	validator validator.Validator,
) AuthService {
	return &authService{userRepo: userRepo,
		outboxRepo:    outboxRepo,
		kafkaProducer: kafkaProducer,
		logger:        logger,
		pool:          pool,
		validator:     validator,
	}
}

func (s *authService) ResetPassword(ctx context.Context, request *pb.ResetPasswordRequest) (*pb.ResetPasswordResponse, error) {
	if err := s.validator.ValidatePassword(request.Password); err != nil {
		return nil, err
	}

	hashedPass, err := bcrypt.GenerateFromPassword([]byte(request.Password), 12)
	if err != nil {
		mylogger.Error(
			ctx,
			s.logger,
			"Error hashing password",
			zap.Error(err),
		)

		return nil, fmt.Errorf("error hashing password: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		mylogger.Error(
			ctx,
			s.logger,
			"Error beginning transaction",
			zap.Error(err),
		)

		return nil, err
	}

	defer func() {
		shutdownCtx := context.WithoutCancel(ctx)
		if err := tx.Rollback(shutdownCtx); err != nil {
			mylogger.Error(ctx, s.logger, "Error rolling back transaction", zap.Error(err))
		}
	}()

	email, err := s.userRepo.ResetPassword(ctx, tx, request.Token, string(hashedPass))
	if err != nil {
		mylogger.Error(
			ctx,
			s.logger,
			"Error resetting password",
			zap.Error(err),
		)

		return nil, fmt.Errorf("error resetting password: %w", err)
	}

	eventPayload := map[string]interface{}{
		"email": email,
		"event": "UserResetPassword",
	}

	payloadBytes, _ := json.Marshal(eventPayload)
	outboxEvent := &outboxDomain.OutboxEvent{
		AggregateType: "User",
		AggregateID:   email,
		EventType:     "UserResetPassword",
		Payload:       payloadBytes,
		Topic:         "user_events",
	}

	if err := s.outboxRepo.SaveOutboxEvent(ctx, tx, outboxEvent); err != nil {
		mylogger.Error(
			ctx,
			s.logger,
			"Error saving outbox event",
			zap.Error(err),
		)

		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction failed: %w", err)
	}

	return &pb.ResetPasswordResponse{Success: true}, nil
}

func (s *authService) ForgotPassword(ctx context.Context, request *pb.ForgotPasswordRequest) (*pb.ForgotPasswordResponse, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("error reading bytes: %v", err)
	}

	forgotPasswordToken := base64.RawURLEncoding.EncodeToString(b)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		mylogger.Error(
			ctx,
			s.logger,
			"Failed to begin tx",
			zap.Error(err),
		)

		return nil, fmt.Errorf("failed to begin tx: %w", err)
	}
	defer func() {
		shutdownCtx := context.WithoutCancel(ctx)
		if err := tx.Rollback(shutdownCtx); err != nil {
			mylogger.Error(ctx, s.logger, "Error rolling back transaction", zap.Error(err))
		}
	}()

	if err := s.userRepo.SetForgotPasswordToken(ctx, tx, request.Email, forgotPasswordToken); err != nil {
		mylogger.Error(
			ctx,
			s.logger,
			"Error saving token",
			zap.String("method_name", "ForgotPassword"),
			zap.Error(err),
		)

		return nil, fmt.Errorf("error saving token: %w", err)
	}

	eventPayload := map[string]interface{}{
		"email":                 request.Email,
		"forgot_password_token": forgotPasswordToken,
		"event":                 "UserForgotPassword",
	}

	payloadBytes, _ := json.Marshal(eventPayload)
	outboxEvent := &outboxDomain.OutboxEvent{
		AggregateType: "User",
		AggregateID:   request.Email,
		EventType:     "UserForgotPassword",
		Payload:       payloadBytes,
		Topic:         "user_events",
	}

	if err := s.outboxRepo.SaveOutboxEvent(ctx, tx, outboxEvent); err != nil {
		mylogger.Error(
			ctx,
			s.logger,
			"Error saving outbox event",
			zap.Error(err),
		)

		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction failed: %w", err)
	}

	return &pb.ForgotPasswordResponse{
		Success: true,
		Message: "Reset link is sent to your email",
	}, nil
}

func (s *authService) Verify(ctx context.Context, request *pb.VerifyRequest) (*pb.VerifyResponse, error) {
	err := s.userRepo.VerifyUser(ctx, request.Token)
	if err != nil {
		mylogger.Error(
			ctx,
			s.logger,
			"Error verifying user",
			zap.String("method_name", "Verify"),
			zap.Error(err),
		)

		return nil, fmt.Errorf("error verifying user: %w", err)
	}

	return &pb.VerifyResponse{
		Success: true,
	}, nil
}

func (s *authService) Logout(ctx context.Context, request *pb.LogoutRequest) (*pb.LogoutResponse, error) {
	err := s.userRepo.DeleteSessionByToken(ctx, request.RefreshToken)
	if err != nil {
		mylogger.Error(
			ctx,
			s.logger,
			"Error deleting session",
			zap.String("method_name", "Logout"),
			zap.Error(err),
		)

		return nil, fmt.Errorf("error deleting session: %w", err)
	}

	return &pb.LogoutResponse{
		Success: true,
	}, nil
}

func (s *authService) Refresh(ctx context.Context, request *pb.RefreshRequest) (*pb.RefreshResponse, error) {
	_, err := utils.ValidateToken(request.RefreshToken, true)
	if err != nil {
		mylogger.Error(
			ctx,
			s.logger,
			"Error validating token",
			zap.Error(err),
		)

		return nil, fmt.Errorf("error validating token: %w", err)
	}

	session, err := s.userRepo.FindSessionByToken(ctx, request.RefreshToken)
	if err != nil {
		mylogger.Error(
			ctx,
			s.logger,
			"Error finding session",
			zap.Error(err),
		)

		return nil, err
	}

	if session.ExpiresAt.Before(time.Now()) {
		if err := s.userRepo.DeleteSessionByID(ctx, session.ID); err != nil {
			mylogger.Warn(
				ctx,
				s.logger,
				"Session expired",
				zap.Int64("session_id", session.ID),
			)

			return nil, err
		}

		return nil, fmt.Errorf("token expired")
	}

	if err := s.userRepo.DeleteSessionByID(ctx, session.ID); err != nil {
		mylogger.Warn(
			ctx,
			s.logger,
			"Error deleting session by id",
			zap.Int64("session_id", session.ID),
		)

		return nil, err
	}

	user, err := s.userRepo.FindUserByID(ctx, session.UserID)
	if err != nil {
		mylogger.Warn(
			ctx,
			s.logger,
			"Error finding user by id",
			zap.Int64("user_id", session.UserID),
		)

		return nil, err
	}

	newAccess, newRefresh, err := utils.GenerateTokens(session.UserID, user.IsActivated)
	if err != nil {
		mylogger.Error(
			ctx,
			s.logger,
			"Error generating tokens",
			zap.Int64("user_id", session.UserID),
			zap.Bool("is_activated", user.IsActivated),
		)

		return nil, fmt.Errorf("error generating tokens: %w", err)
	}

	newSession := domain.RefreshSession{
		UserID:    session.UserID,
		Token:     newRefresh,
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
	}
	if err := s.userRepo.SaveSessionToDB(ctx, &newSession); err != nil {
		return nil, fmt.Errorf("error saving session to db: %w", err)
	}

	return &pb.RefreshResponse{
		AccessToken:  newAccess,
		RefreshToken: newRefresh,
	}, nil
}

func (s *authService) GetUserInfo(ctx context.Context, id int64) (*domain.User, error) {
	res, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		mylogger.Error(
			ctx,
			s.logger,
			"Error finding user by id",
			zap.Error(err),
			zap.Int64("user_id", id),
		)

		return nil, err
	}

	return res, nil
}

func (s *authService) Validate(ctx context.Context, token string) (*pb.ValidateResponse, error) {
	claims, err := utils.ValidateToken(token, false)
	if err != nil {
		mylogger.Warn(
			ctx,
			s.logger,
			"Error validating token",
			zap.Error(err),
			zap.String("token", token),
		)

		return nil, fmt.Errorf("invalid token: %w", err)
	}

	return &pb.ValidateResponse{
		UserId:      claims.UserID,
		IsActivated: claims.IsActivated,
	}, nil
}

func (s *authService) Register(ctx context.Context, email, password string) (*domain.User, error) {
	if err := s.validator.ValidatePassword(password); err != nil {
		return nil, err
	}

	hashedPass, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		mylogger.Error(
			ctx,
			s.logger,
			"Error registering user",
			zap.String("email", email),
			zap.Error(err),
		)

		return nil, fmt.Errorf("error hashing password: %w", err)
	}

	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("error reading bytes: %w", err)
	}

	activationToken := base64.RawURLEncoding.EncodeToString(b)

	user := &domain.User{
		Email:           email,
		Password:        string(hashedPass),
		ActivationToken: activationToken,
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		mylogger.Error(
			ctx,
			s.logger,
			"Error starting transaction",
			zap.Error(err),
		)

		return nil, fmt.Errorf("error starting transaction: %w", err)
	}
	defer func() {
		cleanupCtx := context.WithoutCancel(ctx)
		err := tx.Rollback(cleanupCtx)

		if err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			mylogger.Warn(
				ctx,
				s.logger,
				"Error rolling back transaction",
				zap.Error(err),
				zap.String("method_name", "Register"),
				zap.String("service", "auth_service"),
			)
		}
	}()

	result, err := s.userRepo.Create(ctx, tx, user)

	if err != nil {
		if errors.Is(err, repository.ErrUserAlreadyExists) {
			mylogger.Info(
				ctx,
				s.logger,
				"User already exists",
				zap.String("email", email),
			)

			return nil, err
		}

		mylogger.Error(
			ctx,
			s.logger,
			"Error creating user",
			zap.String("email", user.Email),
			zap.Error(err),
		)

		return nil, fmt.Errorf("error creating user: %w", err)
	}

	userData := map[string]interface{}{
		"user_id":          result.ID,
		"email":            result.Email,
		"activation_token": result.ActivationToken,
		"event_id":         result.ID,
	}

	eventEnvelope := map[string]any{
		"event":   "UserRegistered",
		"payload": userData,
	}

	payloadBytes, err := json.Marshal(eventEnvelope)
	if err != nil {
		mylogger.Warn(
			ctx,
			s.logger,
			"Failed to marshal event envelope",
			zap.Error(err),
		)

		return nil, err
	}

	outboxEvent := &outboxDomain.OutboxEvent{
		AggregateType: "User",
		AggregateID:   fmt.Sprintf("%d", result.ID),
		EventType:     "UserRegistered",
		Payload:       payloadBytes,
		Topic:         "user_events",
	}

	if err := s.outboxRepo.SaveOutboxEvent(ctx, tx, outboxEvent); err != nil {
		mylogger.Error(
			ctx,
			s.logger,
			"Error saving outbox event",
			zap.Error(err),
		)

		return nil, fmt.Errorf("failed to save outbox event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return result, nil
}

func (s *authService) Login(ctx context.Context, email, password string) (string, string, error) {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		mylogger.Warn(
			ctx,
			s.logger,
			"Error getting by email",
			zap.Error(err),
			zap.String("email", email),
		)

		return "", "", fmt.Errorf("invalid credentials")
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		mylogger.Warn(
			ctx,
			s.logger,
			"Invalid credentials",
		)

		return "", "", fmt.Errorf("invalid credentials")
	}

	accessToken, refreshToken, err := utils.GenerateTokens(user.ID, user.IsActivated)
	if err != nil {
		mylogger.Warn(
			ctx,
			s.logger,
			"Failed to generate tokens",
			zap.Error(err),
		)

		return "", "", fmt.Errorf("failed to generate tokens: %v", err)
	}

	session := &domain.RefreshSession{
		UserID:    user.ID,
		Token:     refreshToken,
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
	}

	err = s.userRepo.SaveSessionToDB(ctx, session)
	if err != nil {
		mylogger.Warn(
			ctx,
			s.logger,
			"Failed to save session to db",
			zap.Error(err),
		)

		return "", "", fmt.Errorf("failed to save session to db: %v", err)
	}

	return accessToken, refreshToken, nil
}
