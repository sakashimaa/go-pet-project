package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sakashimaa/go-pet-project/order/internal/domain"
	outboxDomain "github.com/sakashimaa/go-pet-project/pkg/domain"
	"github.com/sakashimaa/go-pet-project/pkg/mylogger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type OrderRepository interface {
	SaveUserDuplication(ctx context.Context, event *domain.UserRegisteredEvent) error
	CreateOrder(ctx context.Context, tx pgx.Tx, order *domain.Order) error
	ChangeOrderStatus(ctx context.Context, tx pgx.Tx, orderID int64, status string) error
	GetAllItemsOfOrder(ctx context.Context, tx pgx.Tx, orderID int64) ([]outboxDomain.OrderItem, error)
}

type orderRepo struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
	tracer trace.Tracer
}

func NewOrderRepository(pool *pgxpool.Pool, logger *zap.Logger) OrderRepository {
	return &orderRepo{
		pool:   pool,
		logger: logger,
		tracer: otel.Tracer("order_repository"),
	}
}

func (r *orderRepo) GetAllItemsOfOrder(ctx context.Context, tx pgx.Tx, orderID int64) ([]outboxDomain.OrderItem, error) {
	ctx, span := r.tracer.Start(ctx, "OrderRepository.GetAllItemsOfOrder")
	defer span.End()

	span.SetAttributes(
		attribute.Int64("order_id", orderID),
	)

	query := `
		SELECT id, product_id, name, price, quantity
		FROM order_items
		WHERE order_id = $1;
	`

	rows, err := tx.Query(ctx, query, orderID)
	if err != nil {
		span.RecordError(err)

		mylogger.Error(
			ctx,
			r.logger,
			"Failed to query order_items",
			zap.Error(err),
		)

		return nil, err
	}
	defer rows.Close()

	var result []outboxDomain.OrderItem
	for rows.Next() {
		var item outboxDomain.OrderItem
		if err := rows.Scan(
			&item.ID,
			&item.ProductID,
			&item.Name,
			&item.Price,
			&item.Quantity,
		); err != nil {
			span.RecordError(err)
			mylogger.Error(
				ctx,
				r.logger,
				"Failed to scan row",
				zap.Error(err),
			)

			return nil, err
		}

		result = append(result, item)
	}

	if err := rows.Err(); err != nil {
		mylogger.Error(
			ctx,
			r.logger,
			"Rows error",
			zap.Error(err),
		)

		return nil, err
	}

	return result, nil
}

func (r *orderRepo) ChangeOrderStatus(ctx context.Context, tx pgx.Tx, orderID int64, status string) error {
	ctx, span := r.tracer.Start(ctx, "OrderRepository.ChangeOrderStatus")
	defer span.End()

	span.SetAttributes(
		attribute.Int64("order_id", orderID),
		attribute.String("status", status),
	)

	query := `
		UPDATE orders
		SET status = $1
		WHERE id = $2;
	`

	commandTag, err := tx.Exec(ctx, query, status, orderID)
	if err != nil {
		span.RecordError(err)

		mylogger.Error(
			ctx,
			r.logger,
			"Failed to update order",
			zap.Error(err),
		)

		return fmt.Errorf("failed to update order: %w", err)
	}

	if commandTag.RowsAffected() == 0 {
		mylogger.Warn(
			ctx,
			r.logger,
			"Order not found",
			zap.Int64("order_id", orderID),
			zap.Error(err),
		)

		return ErrOrderNotFound
	}

	return nil
}

func (r *orderRepo) CreateOrder(ctx context.Context, tx pgx.Tx, order *domain.Order) error {
	ctx, span := r.tracer.Start(ctx, "OrderRepository.CreateOrder")
	defer span.End()

	span.SetAttributes(
		attribute.Int64("order_id", order.UserID),
		attribute.Int("items_count", len(order.Items)),
	)

	queryOrder := `
		INSERT INTO orders (user_id, status, total_sum, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		RETURNING id, created_at, updated_at
	`

	if err := tx.QueryRow(
		ctx,
		queryOrder,
		order.UserID,
		string(order.Status),
		order.TotalSum,
	).Scan(
		&order.ID,
		&order.CreatedAt,
		&order.UpdatedAt,
	); err != nil {
		span.RecordError(err)

		mylogger.Warn(
			ctx,
			r.logger,
			"Failed to insert order",
			zap.Error(err),
		)

		return err
	}

	queryItem := `
		INSERT INTO order_items (order_id, product_id, name, price, quantity)
		VALUES ($1, $2, $3, $4, $5)
	`

	for _, item := range order.Items {
		_, err := tx.Exec(
			ctx,
			queryItem,
			order.ID,
			item.ProductID,
			item.Name,
			item.Price,
			item.Quantity,
		)
		if err != nil {
			span.RecordError(err)

			mylogger.Error(
				ctx,
				r.logger,
				"Failed to insert item",
				zap.Error(err),
			)

			return fmt.Errorf("failed to insert order item: %v", err)
		}
	}

	return nil
}

func (r *orderRepo) SaveUserDuplication(ctx context.Context, event *domain.UserRegisteredEvent) error {
	ctx, span := r.tracer.Start(ctx, "OrderRepository.SaveUserDuplication")
	defer span.End()

	span.SetAttributes(
		attribute.Int64("user_id", event.UserID),
		attribute.String("email", event.Email),
	)

	query := `
		INSERT INTO users (id, email)
		VALUES ($1, $2)
	`

	_, err := r.pool.Exec(ctx, query, event.UserID, event.Email)
	if err != nil {
		var pgError *pgconn.PgError
		if errors.As(err, &pgError) {
			if pgError.Code == "23505" {
				mylogger.Warn(
					ctx,
					r.logger,
					"User already exists, skipping",
					zap.Int64("user_id", event.UserID),
				)

				return nil
			}
		}

		span.RecordError(err)

		mylogger.Error(
			ctx,
			r.logger,
			"Error inserting into users",
			zap.Int64("user_id", event.UserID),
			zap.Error(err),
		)

		return err
	}

	return nil
}
