package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	generalDomain "github.com/sakashimaa/go-pet-project/pkg/domain"
	"github.com/sakashimaa/go-pet-project/pkg/mylogger"
	outboxDomain "github.com/sakashimaa/go-pet-project/pkg/outbox/domain"
	"github.com/sakashimaa/go-pet-project/pkg/outbox/worker"
	"github.com/sakashimaa/go-pet-project/product/internal/domain"
	"github.com/sakashimaa/go-pet-project/product/internal/repository"
	"go.uber.org/zap"
)

type ProductService interface {
	Create(ctx context.Context, product *domain.Product) (int64, error)
	FindByID(ctx context.Context, id int64) (*domain.Product, error)
	List(ctx context.Context, limit, offset int64, search string) ([]domain.Product, int64, error)
	DecreaseStock(ctx context.Context, id, quantity int64) (string, error)
	Delete(ctx context.Context, id int64) error
	ReserveProduct(ctx context.Context, event *domain.OrderCreatedEvent) error
	ReturnStock(ctx context.Context, event *generalDomain.OrderCancelledEvent) error
}

type productService struct {
	productRepo repository.ProductRepository
	outboxRepo  worker.OutboxRepository
	pool        *pgxpool.Pool
	logger      *zap.Logger
}

func NewProductService(
	productRepo repository.ProductRepository,
	outboxRepo worker.OutboxRepository,
	pool *pgxpool.Pool,
	logger *zap.Logger,
) ProductService {
	return &productService{
		productRepo: productRepo,
		outboxRepo:  outboxRepo,
		pool:        pool,
		logger:      logger,
	}
}

func (s *productService) ReturnStock(ctx context.Context, event *generalDomain.OrderCancelledEvent) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		mylogger.Warn(
			ctx,
			s.logger,
			"Failed to begin transaction",
			zap.Error(err),
		)

		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		shutdownCtx := context.WithoutCancel(ctx)
		if err := tx.Rollback(shutdownCtx); err != nil {
			mylogger.Warn(shutdownCtx, s.logger, "Failed to rollback transaction", zap.Error(err))
		}
	}()

	for _, item := range event.Items {
		if err := s.productRepo.IncreaseStock(ctx, tx, item.ProductID, item.Quantity); err != nil {
			mylogger.Warn(ctx,
				s.logger,
				"Failed to increase stock",
				zap.Int64("product_id", item.ProductID),
				zap.Int32("quantity", item.Quantity),
			)

			return err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		mylogger.Error(ctx, s.logger, "Failed to commit transaction", zap.Error(err))
		return err
	}

	return nil
}

func (s *productService) ReserveProduct(ctx context.Context, event *domain.OrderCreatedEvent) error {
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

	var total int64
	for _, item := range event.Items {
		price, err := s.productRepo.DecreaseStock(ctx, tx, item.ProductID, item.Quantity)
		total += price * item.Quantity

		if err != nil {
			if errors.Is(err, repository.ErrInsufficientStock) {
				mylogger.Warn(ctx, s.logger, "Insufficient stock", zap.Int64("product_id", item.ProductID))
				return err
			}

			mylogger.Warn(ctx, s.logger, "Error processing order created", zap.Error(err))
			return err
		}
	}

	successEvent := domain.InventoryReservedEvent{
		OrderID:    event.OrderID,
		UserID:     event.UserID,
		Amount:     total,
		ReservedAt: time.Now(),
	}

	payloadMap := map[string]any{
		"event":   "InventoryReserved",
		"payload": successEvent,
	}
	payloadBytes, _ := json.Marshal(payloadMap)

	outboxEvent := &outboxDomain.OutboxEvent{
		Topic:         "payment_events",
		AggregateType: "Inventory",
		AggregateID:   fmt.Sprintf("%d", event.OrderID),
		EventType:     "InventoryReserved",
		Payload:       payloadBytes,
	}

	if err := s.outboxRepo.SaveOutboxEvent(ctx, tx, outboxEvent); err != nil {
		return fmt.Errorf("failed to save outbox event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	mylogger.Info(ctx, s.logger, "Product reserved successfully", zap.Int64("order_id", event.OrderID))
	return nil
}

func (s *productService) Delete(ctx context.Context, id int64) error {
	err := s.productRepo.DeleteByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrProductNotFound) {
			s.logger.Warn("product not found", zap.Int64("product_id", id))
			return err
		}

		s.logger.Error("error deleting product", zap.Error(err))
		return err
	}

	return nil
}

func (s *productService) DecreaseStock(ctx context.Context, id, quantity int64) (string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		mylogger.Warn(
			ctx,
			s.logger,
			"Failed to begin transaction",
			zap.Error(err),
		)

		return "", err
	}

	_, err = s.productRepo.DecreaseStock(ctx, tx, id, quantity)
	if err != nil {
		if errors.Is(err, repository.ErrInsufficientStock) {
			s.logger.Warn("insufficient stock",
				zap.Int64("product_id", id),
				zap.Int64("quantity", quantity),
			)
			return "", err
		}

		s.logger.Error("error decreasing stock", zap.Error(err))
		return "", err
	}

	return "success", nil
}

func (s *productService) Create(ctx context.Context, product *domain.Product) (int64, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		mylogger.Error(ctx, s.logger, "Error starting transaction", zap.Error(err))
		return 0, err
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

	id, err := s.productRepo.Create(ctx, tx, product)

	if err != nil {
		mylogger.Error(ctx, s.logger, "create error", zap.Error(err))
		return 0, fmt.Errorf("error creating product: %w", err)
	}

	eventPayload := map[string]interface{}{
		"product_id": id,
		"event":      "ProductCreated",
	}

	payloadBytes, err := json.Marshal(eventPayload)
	if err != nil {
		return 0, fmt.Errorf("event payload marshal error: %w", err)
	}

	outboxEvent := &outboxDomain.OutboxEvent{
		AggregateType: "Product",
		AggregateID:   fmt.Sprintf("%d", id),
		EventType:     "ProductCreated",
		Payload:       payloadBytes,
		Topic:         "product_events",
	}

	if err := s.outboxRepo.SaveOutboxEvent(ctx, tx, outboxEvent); err != nil {
		mylogger.Error(
			ctx,
			s.logger,
			"Error saving outbox event",
			zap.Error(err),
		)

		return 0, fmt.Errorf("failed to save outbox event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		mylogger.Error(
			ctx,
			s.logger,
			"Error commiting transaction",
			zap.Error(err),
		)

		return 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return id, nil
}

func (s *productService) FindByID(ctx context.Context, id int64) (*domain.Product, error) {
	res, err := s.productRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrProductNotFound) {
			s.logger.Warn("product not found", zap.Int64("product_id", id))
			return nil, err
		}

		s.logger.Error("error getting product", zap.Error(err))
		return nil, fmt.Errorf("error getting product by id: %w", err)
	}

	return res, nil
}

func (s *productService) List(ctx context.Context, limit, offset int64, search string) ([]domain.Product, int64, error) {
	list, quantity, err := s.productRepo.List(ctx, limit, offset, search)
	if err != nil {
		s.logger.Error("list error", zap.Error(err))
		return nil, 0, fmt.Errorf("error listing products: %w", err)
	}

	return list, quantity, nil
}
