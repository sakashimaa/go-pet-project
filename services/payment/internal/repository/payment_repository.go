package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sakashimaa/go-pet-project/payment/internal/domain"
	"github.com/sakashimaa/go-pet-project/pkg/mylogger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type PaymentRepository interface {
	Create(ctx context.Context, tx pgx.Tx, payment *domain.Payment) error
	GetOrderByID(ctx context.Context, orderID int64) (*domain.Payment, error)
}

type paymentRepo struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
	tracer trace.Tracer
}

func NewPaymentRepository(pool *pgxpool.Pool, logger *zap.Logger) PaymentRepository {
	return &paymentRepo{
		pool:   pool,
		logger: logger,
		tracer: otel.Tracer("repository/payment_repo"),
	}
}

func (r *paymentRepo) Create(ctx context.Context, tx pgx.Tx, payment *domain.Payment) error {
	ctx, span := r.tracer.Start(ctx, "PaymentRepository.Create")
	defer span.End()

	span.SetAttributes(
		attribute.Int64("order_id", payment.OrderID),
		attribute.Int64("user_id", payment.UserID),
		attribute.Int64("amount", payment.Amount),
	)

	query := `
		INSERT INTO payments (order_id, user_id, amount, status, transaction_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
		RETURNING id, created_at, updated_at
	`

	if err := tx.QueryRow(ctx, query,
		payment.OrderID,
		payment.UserID,
		payment.Amount,
		payment.Status,
		payment.TransactionID,
	).Scan(
		&payment.ID,
		&payment.CreatedAt,
		&payment.UpdatedAt,
	); err != nil {
		span.RecordError(err)

		mylogger.Warn(ctx, r.logger, "Create payment failed", zap.Error(err))

		return err
	}

	return nil
}

func (r *paymentRepo) GetOrderByID(ctx context.Context, orderID int64) (*domain.Payment, error) {
	ctx, span := r.tracer.Start(ctx, "PaymentRepository.GetOrderByID")
	defer span.End()

	query := `
		SELECT id, order_id, status
		FROM payments
		WHERE order_id = $1
	`

	var result domain.Payment
	if err := r.pool.QueryRow(ctx, query, orderID).
		Scan(&result.ID, &result.OrderID, &result.Status); err != nil {
		span.RecordError(err)

		if errors.Is(err, pgx.ErrNoRows) {
			mylogger.Warn(ctx, r.logger, "Order not found", zap.Int64("order_id", orderID))
			return nil, nil
		}

		mylogger.Error(ctx, r.logger, "GetOrderByID failed", zap.Error(err))

		return nil, fmt.Errorf("error getting order by id: %w", err)
	}

	return &result, nil
}
