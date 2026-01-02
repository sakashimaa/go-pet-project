package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sakashimaa/go-pet-project/pkg/mylogger"
	"github.com/sakashimaa/go-pet-project/pkg/outbox/domain"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type OutboxRepository interface {
	SaveOutboxEvent(ctx context.Context, tx pgx.Tx, event *domain.OutboxEvent) error
	GetUnpublishedEvents(ctx context.Context, tx pgx.Tx, batchSize int) ([]*domain.OutboxEvent, error)
	MarkEventUnpublished(ctx context.Context, tx pgx.Tx, eventID int64) error
	MarkEventPublished(ctx context.Context, tx pgx.Tx, eventID int64) error
	MarkEventFailed(ctx context.Context, tx pgx.Tx, eventID int64, error string) error
}

type KafkaProducer interface {
	ProduceMessage(ctx context.Context, topic string, message interface{}) error
}

type OutboxProcessor struct {
	pool          *pgxpool.Pool
	repo          OutboxRepository
	kafkaProducer KafkaProducer
	logger        *zap.Logger
	batchSize     int
	interval      time.Duration
	tracer        trace.Tracer
}

func NewOutboxProcessor(
	pool *pgxpool.Pool,
	repo OutboxRepository,
	producer KafkaProducer,
	logger *zap.Logger,
) *OutboxProcessor {
	return &OutboxProcessor{
		pool:          pool,
		repo:          repo,
		kafkaProducer: producer,
		logger:        logger,
		batchSize:     50,
		interval:      500 * time.Millisecond,
		tracer:        otel.Tracer("outbox-worker"),
	}
}

func (p *OutboxProcessor) Start(ctx context.Context) {
	mylogger.Info(
		ctx,
		p.logger,
		"Starting outbox processor",
	)

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			mylogger.Info(
				ctx,
				p.logger,
				"Outbox processor stopping",
			)

			return
		case <-ticker.C:
			if err := p.processBatch(ctx); err != nil {
				mylogger.Error(
					ctx,
					p.logger,
					"Error processing outbox batch",
					zap.Error(err),
				)
			}
		}
	}
}

func (p *OutboxProcessor) processBatch(ctx context.Context) error {
	ctx, span := p.tracer.Start(ctx, "OutboxProcessor.processBatch")
	defer span.End()

	tx, err := p.pool.Begin(ctx)
	if err != nil {
		mylogger.Error(
			ctx,
			p.logger,
			"outbox worker failed to begin transaction",
			zap.Error(err),
		)

		return fmt.Errorf("error beginning transaction: %w", err)
	}
	defer func() {
		cleanupCtx := context.WithoutCancel(ctx)

		err := tx.Rollback(cleanupCtx)
		if err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			mylogger.Error(
				cleanupCtx,
				p.logger,
				"Outbox worker failed to rollback transaction",
				zap.Error(err),
				zap.String("method_name", "processBatch"),
			)
		}
	}()

	events, err := p.repo.GetUnpublishedEvents(ctx, tx, p.batchSize)
	if err != nil {
		return err
	}

	if len(events) == 0 {
		return nil
	}

	mylogger.Info(
		ctx,
		p.logger,
		"Processing outbox events",
		zap.Int("count", len(events)),
	)

	for _, event := range events {
		var payloadMap map[string]any
		if err := json.Unmarshal(event.Payload, &payloadMap); err != nil {
			mylogger.Error(
				ctx,
				p.logger,
				"outbox worker unmarshal event payload failed",
				zap.Int64("id", event.Id),
				zap.Error(err),
			)

			_ = p.repo.MarkEventFailed(ctx, tx, event.Id, err.Error())
			continue
		}

		payloadMap["event_id"] = event.Id

		err = p.kafkaProducer.ProduceMessage(
			ctx,
			event.Topic,
			payloadMap,
		)
		if err != nil {
			mylogger.Error(
				ctx,
				p.logger,
				"outbox worker produce message failed",
				zap.Int64("id", event.Id),
				zap.Error(err),
			)
			if dbErr := p.repo.MarkEventFailed(ctx, tx, event.Id, err.Error()); dbErr != nil {
				mylogger.Error(
					ctx,
					p.logger,
					"outbox worker mark event failed failed",
					zap.Int64("id", event.Id),
					zap.Error(err),
				)
			}
		} else {
			if dbErr := p.repo.MarkEventPublished(ctx, tx, event.Id); dbErr != nil {
				mylogger.Error(
					ctx,
					p.logger,
					"Outbox worker event publishing failed",
					zap.Int64("id", event.Id),
					zap.Error(err),
				)

				return err
			}

			mylogger.Debug(
				ctx,
				p.logger,
				"outbox worker event published successfully",
				zap.Int64("id", event.Id),
			)
		}
	}

	return tx.Commit(ctx)
}
