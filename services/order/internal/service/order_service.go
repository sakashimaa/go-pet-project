package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sakashimaa/go-pet-project/order/internal/domain"
	"github.com/sakashimaa/go-pet-project/order/internal/repository"
	generalDomain "github.com/sakashimaa/go-pet-project/pkg/domain"
	"github.com/sakashimaa/go-pet-project/pkg/mylogger"
	outboxDomain "github.com/sakashimaa/go-pet-project/pkg/outbox/domain"
	"github.com/sakashimaa/go-pet-project/pkg/outbox/worker"
	pb "github.com/sakashimaa/go-pet-project/proto/order"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type OrderService interface {
	HandleUserRegistered(ctx context.Context, event *domain.UserRegisteredEvent) error
	CreateOrder(ctx context.Context, req *pb.CreateOrderRequest) (*pb.CreateOrderResponse, error)
	ChangeOrderStatus(ctx context.Context, event *generalDomain.PaymentSucceededEvent) error
	CancelOrder(ctx context.Context, event *generalDomain.PaymentFailedEvent) error
}

type orderService struct {
	pool       *pgxpool.Pool
	logger     *zap.Logger
	orderRepo  repository.OrderRepository
	outboxRepo worker.OutboxRepository
	tracer     trace.Tracer
}

func NewOrderService(pool *pgxpool.Pool, logger *zap.Logger, orderRepo repository.OrderRepository, outboxRepo worker.OutboxRepository) OrderService {
	return &orderService{
		pool:       pool,
		logger:     logger,
		orderRepo:  orderRepo,
		outboxRepo: outboxRepo,
		tracer:     otel.Tracer("order_service"),
	}
}

func (s *orderService) CancelOrder(ctx context.Context, event *generalDomain.PaymentFailedEvent) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		mylogger.Warn(
			ctx,
			s.logger,
			"Failed to begin transaction",
			zap.Error(err),
		)

		return err
	}
	defer func() {
		shutdownCtx := context.WithoutCancel(ctx)

		if err := tx.Rollback(shutdownCtx); err != nil {
			mylogger.Warn(shutdownCtx, s.logger, "Failed to rollback transaction", zap.Error(err))
		}
	}()

	err = s.orderRepo.ChangeOrderStatus(ctx, tx, event.OrderID, "cancelled")
	if err != nil {
		if errors.Is(err, repository.ErrOrderNotFound) {
			mylogger.Warn(
				ctx,
				s.logger,
				"Order not found",
				zap.Int64("order_id", event.OrderID),
			)

			return fmt.Errorf("order %d not found", event.OrderID)
		}

		mylogger.Warn(
			ctx,
			s.logger,
			"Cancel order failed",
			zap.Error(err),
		)

		return fmt.Errorf("failed to cancel order: %w", err)
	}

	orderItems, err := s.orderRepo.GetAllItemsOfOrder(ctx, tx, event.OrderID)
	if err != nil {
		mylogger.Error(
			ctx,
			s.logger,
			"Failed to query items of order",
			zap.Int64("order_id", event.OrderID),
			zap.Error(err),
		)

		return fmt.Errorf("failed to query items of order: %w", err)
	}

	err = s.emitEvent(ctx, tx, "OrderCancelled", &generalDomain.OrderCancelledEvent{
		OrderID: event.OrderID,
		Items:   orderItems,
	})
	if err != nil {
		return fmt.Errorf("failed to emit event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		mylogger.Error(ctx, s.logger, "Failed to commit transaction", zap.Error(err))
		return err
	}

	return nil
}

func (s *orderService) ChangeOrderStatus(ctx context.Context, event *generalDomain.PaymentSucceededEvent) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		mylogger.Error(
			ctx,
			s.logger,
			"Failed to begin transaction",
			zap.Error(err),
		)

		return fmt.Errorf("failed to begin transaction")
	}
	defer func() {
		shutdownCtx := context.WithoutCancel(ctx)
		if err := tx.Rollback(shutdownCtx); err != nil {
			mylogger.Error(
				shutdownCtx,
				s.logger,
				"Failed to rollback transaction",
				zap.Error(err),
			)
		}
	}()

	err = s.orderRepo.ChangeOrderStatus(ctx, tx, event.OrderID, "paid")
	if err != nil {
		if errors.Is(err, repository.ErrOrderNotFound) {
			mylogger.Warn(
				ctx,
				s.logger,
				"Order not found",
				zap.Int64("order_id", event.OrderID),
			)

			return fmt.Errorf("order %d not found", event.OrderID)
		}

		mylogger.Error(
			ctx,
			s.logger,
			"Failed to update order status",
			zap.Int64("order_id", event.OrderID),
			zap.Error(err),
		)

		return fmt.Errorf("failed to update order status: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		mylogger.Error(
			ctx,
			s.logger,
			"Failed to commit transaction",
			zap.Error(err),
		)

		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (s *orderService) CreateOrder(ctx context.Context, req *pb.CreateOrderRequest) (*pb.CreateOrderResponse, error) {
	items := make([]domain.OrderItem, 0, len(req.Items))
	for _, item := range req.Items {
		items = append(items, domain.OrderItem{
			ProductID: item.ProductId,
			Name:      item.Name,
			Price:     item.Price,
			Quantity:  item.Quantity,
		})
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %v", err)
	}
	defer func() {
		shutdownCtx := context.WithoutCancel(ctx)
		err := tx.Rollback(shutdownCtx)

		if err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			mylogger.Warn(
				shutdownCtx,
				s.logger,
				"Error rolling back transaction",
				zap.Error(err),
			)
		}
	}()

	order := &domain.Order{
		UserID: req.UserId,
		Status: domain.OrderStatusNew,
		Items:  items,
	}

	order.CalculateTotal()

	err = s.orderRepo.CreateOrder(ctx, tx, order)
	if err != nil {
		mylogger.Error(
			ctx,
			s.logger,
			"Failed to create order",
			zap.Int64("user_id", req.UserId),
			zap.Error(err),
		)

		return nil, fmt.Errorf("failed to create order: %v", err)
	}

	eventItems := make([]map[string]any, len(items))
	for i, item := range items {
		eventItems[i] = map[string]any{
			"product_id": item.ProductID,
			"quantity":   item.Quantity,
		}
	}

	orderData := map[string]any{
		"order_id": order.ID,
		"event_id": order.ID,
		"user_id":  order.UserID,
		"items":    eventItems,
	}

	eventEnvelope := map[string]any{
		"event":   "OrderCreated",
		"payload": orderData,
	}

	payloadBytes, err := json.Marshal(eventEnvelope)
	if err != nil {
		mylogger.Warn(
			ctx,
			s.logger,
			"Failed to marshal event envelope",
			zap.Error(err),
		)

		return nil, fmt.Errorf("failed to marshal event envelope: %v", err)
	}

	outboxEvent := &outboxDomain.OutboxEvent{
		AggregateType: "Order",
		AggregateID:   fmt.Sprintf("%d", order.ID),
		EventType:     "OrderCreated",
		Payload:       payloadBytes,
		Headers:       nil,
		Topic:         "order_events",
	}

	if err := s.outboxRepo.SaveOutboxEvent(ctx, tx, outboxEvent); err != nil {
		mylogger.Error(
			ctx,
			s.logger,
			"Failed to save outbox event",
			zap.Error(err),
		)

		return nil, fmt.Errorf("failed to save outbox event: %v", err)
	}

	if err := tx.Commit(ctx); err != nil {
		mylogger.Error(
			ctx,
			s.logger,
			"Failed to commit transaction",
			zap.Error(err),
		)

		return nil, fmt.Errorf("failed to commit transaction: %v", err)
	}

	return &pb.CreateOrderResponse{OrderId: order.ID}, nil
}

func (s *orderService) HandleUserRegistered(ctx context.Context, event *domain.UserRegisteredEvent) error {
	ctx, span := s.tracer.Start(ctx, "OrderService.HandleUserRegistered")
	defer span.End()

	span.SetAttributes(
		attribute.Int64("user_id", event.UserID),
	)

	if err := s.orderRepo.SaveUserDuplication(ctx, event); err != nil {
		span.RecordError(err)

		mylogger.Warn(
			ctx,
			s.logger,
			"Failed to save user",
			zap.Error(err),
		)

		return err
	}

	mylogger.Info(
		ctx,
		s.logger,
		"User saved successfully",
		zap.Int64("user_id", event.UserID),
	)

	return nil
}

func (s *orderService) emitEvent(ctx context.Context, tx pgx.Tx, eventType string, payload any) error {
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
