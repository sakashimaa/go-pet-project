package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sakashimaa/go-pet-project/payment/internal/domain"
	"github.com/sakashimaa/go-pet-project/payment/internal/repository"
	generalDomain "github.com/sakashimaa/go-pet-project/pkg/domain"
	"github.com/sakashimaa/go-pet-project/pkg/mylogger"
	outboxDomain "github.com/sakashimaa/go-pet-project/pkg/outbox/domain"
	"github.com/sakashimaa/go-pet-project/pkg/outbox/worker"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type PaymentService interface {
	ProcessPayment(ctx context.Context, event domain.InventoryReservedEvent) error
}

type paymentService struct {
	pool        *pgxpool.Pool
	paymentRepo repository.PaymentRepository
	outboxRepo  worker.OutboxRepository
	logger      *zap.Logger
	tracer      trace.Tracer
}

func NewPaymentService(
	pool *pgxpool.Pool,
	paymentRepo repository.PaymentRepository,
	outboxRepo worker.OutboxRepository,
	logger *zap.Logger,
) PaymentService {
	return &paymentService{
		pool:        pool,
		paymentRepo: paymentRepo,
		outboxRepo:  outboxRepo,
		logger:      logger,
		tracer:      otel.Tracer("service/payment_service"),
	}
}

func (s *paymentService) ProcessPayment(ctx context.Context, event domain.InventoryReservedEvent) error {
	ctx, span := s.tracer.Start(ctx, "PaymentService.ProcessPayment")
	defer span.End()

	mylogger.Info(
		ctx,
		s.logger,
		"Processing payment",
		zap.Int64("order_id", event.OrderID),
		zap.Int64("user_id", event.UserID),
	)

	existingPayment, err := s.paymentRepo.GetOrderByID(ctx, event.OrderID)
	if err != nil {
		mylogger.Warn(
			ctx,
			s.logger,
			"Get order by id failed",
			zap.Int64("order_id", event.OrderID),
			zap.Error(err),
		)

		return err
	}
	if existingPayment != nil {
		mylogger.Warn(
			ctx,
			s.logger,
			"Payment already exists for this order",
			zap.Int64("order_id", event.OrderID),
		)

		return nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		mylogger.Error(
			ctx,
			s.logger,
			"Error beginning transaction",
			zap.Error(err),
		)

		return fmt.Errorf("error beginning transaction: %w", err)
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

	var status string
	var eventType string
	var eventPayload any

	if event.OrderID%2 == 0 {
		status = "FAIL"
		eventType = "PaymentFailed"
		eventPayload = generalDomain.PaymentFailedEvent{
			OrderID:  event.OrderID,
			Amount:   event.Amount,
			FailedAt: time.Now(),
		}
	} else {
		status = "PAID"
		eventType = "PaymentSucceeded"
		eventPayload = generalDomain.PaymentSucceededEvent{
			OrderID: event.OrderID,
			Amount:  event.Amount,
			PaidAt:  time.Now(),
		}
	}

	payment := &domain.Payment{
		OrderID:       event.OrderID,
		UserID:        event.UserID,
		Amount:        event.Amount,
		Status:        status,
		TransactionID: uuid.New().String(),
	}

	if err := s.paymentRepo.Create(ctx, tx, payment); err != nil {
		mylogger.Warn(ctx, s.logger, "Payment create failed", zap.Error(err))
		return err
	}

	if err := s.emitEvent(ctx, tx, eventType, eventPayload); err != nil {
		mylogger.Warn(
			ctx,
			s.logger,
			"Failed to emit event",
			zap.Error(err),
		)

		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	mylogger.Info(
		ctx,
		s.logger,
		"ProcessPayment finished",
		zap.Int64("order_id", event.OrderID),
	)

	return nil
}

func (s *paymentService) emitEvent(ctx context.Context, tx pgx.Tx, eventType string, payload any) error {
	wrapper := map[string]any{
		"event":   eventType,
		"payload": payload,
	}

	wrapperBytes, err := json.Marshal(wrapper)
	if err != nil {
		return fmt.Errorf("failed to marshal wrapper: %w", err)
	}

	outboxEvent := &outboxDomain.OutboxEvent{
		Topic:   "payment_events",
		Payload: wrapperBytes,
	}

	return s.outboxRepo.SaveOutboxEvent(ctx, tx, outboxEvent)
}
