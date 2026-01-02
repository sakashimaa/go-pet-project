package service

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sakashimaa/go-pet-project/notification/internal/domain"
	"github.com/sakashimaa/go-pet-project/notification/internal/infrastructure/email"
	"github.com/sakashimaa/go-pet-project/pkg/mylogger"
	outboxUtils "github.com/sakashimaa/go-pet-project/pkg/outbox/utils"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type NotificationService struct {
	emailSender email.Sender
	logger      *zap.Logger
	pool        *pgxpool.Pool
	tracer      trace.Tracer
}

func NewNotificationService(emailSender email.Sender, logger *zap.Logger, pool *pgxpool.Pool) *NotificationService {
	return &NotificationService{
		emailSender: emailSender,
		logger:      logger,
		pool:        pool,
		tracer:      otel.Tracer("notification-service"),
	}
}

func (s *NotificationService) HandleUserRegistered(ctx context.Context, event domain.UserRegisteredEvent) error {
	ctx, span := s.tracer.Start(ctx, "NotificationService.HandleUserRegistered")
	defer span.End()

	span.SetAttributes(attribute.Int64("event_id", event.EventID))

	return outboxUtils.ProcessWithDeduplication(ctx, s.pool, s.logger, event.EventID, func() error {
		return s.emailSender.SendActivationEmail(ctx, event.Email, event.ActivationToken)
	})
}

func (s *NotificationService) HandleUserForgotPassword(ctx context.Context, event domain.UserForgotPasswordEvent) error {
	ctx, span := s.tracer.Start(ctx, "NotificationService.HandleUserForgotPassword")
	defer span.End()

	span.SetAttributes(attribute.String("email", event.Email))

	return outboxUtils.ProcessWithDeduplication(ctx, s.pool, s.logger, event.EventID, func() error {
		return s.emailSender.SendForgotPasswordEmail(ctx, event.Email, event.ForgotPasswordToken)
	})
}

func (s *NotificationService) HandleUserResetPassword(ctx context.Context, event domain.UserResetPasswordEvent) error {
	mylogger.Info(
		ctx,
		s.logger,
		"Sending reset password",
		zap.String("to", event.Email),
	)

	err := s.emailSender.SendResetPasswordEmail(ctx, event.Email)
	if err != nil {
		mylogger.Error(
			ctx,
			s.logger,
			"Error sending reset password email",
			zap.String("to", event.Email),
		)

		return err
	}

	mylogger.Info(
		ctx,
		s.logger,
		"Reset password link sent successfully",
		zap.String("to", event.Email),
	)
	return nil
}
