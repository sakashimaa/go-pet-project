package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sakashimaa/go-pet-project/pkg/outbox/domain"
	"github.com/sakashimaa/go-pet-project/pkg/outbox/worker"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type outboxRepo struct {
	pool   *pgxpool.Pool
	tracer trace.Tracer
	logger *zap.Logger
}

func NewOutboxRepository(pool *pgxpool.Pool, logger *zap.Logger) worker.OutboxRepository {
	return &outboxRepo{
		pool:   pool,
		tracer: otel.Tracer("contract/outbox_repo"),
		logger: logger,
	}
}

func (r *outboxRepo) MarkEventFailed(ctx context.Context, tx pgx.Tx, eventID int64, errMsg string) error {
	ctx, span := r.tracer.Start(ctx, "OutboxRepo.MarkEventFailed")
	defer span.End()

	span.SetAttributes(
		attribute.Int64("event_id", eventID),
		attribute.String("outbox.error_message", errMsg),
	)

	query := `
		UPDATE outbox
		SET published_at = NULL,
			last_error = $1,
			attempts = attempts + 1
		WHERE id = $2;
	`

	_, err := tx.Exec(ctx, query, errMsg, eventID)
	if err != nil {
		span.RecordError(err)
	}

	return err
}

func (r *outboxRepo) MarkEventPublished(ctx context.Context, tx pgx.Tx, eventID int64) error {
	ctx, span := r.tracer.Start(ctx, "OutboxRepository.MarkEventPublished")
	defer span.End()

	span.SetAttributes(
		attribute.Int64("event_id", eventID),
	)

	query := `
		UPDATE outbox
		SET published_at = NOW(), last_error = NULL
		WHERE id = $1;
	`

	_, err := tx.Exec(ctx, query, eventID)

	if err != nil {
		span.RecordError(err)
	}

	return err
}

func (r *outboxRepo) MarkEventUnpublished(ctx context.Context, tx pgx.Tx, eventID int64) error {
	ctx, span := r.tracer.Start(ctx, "OutboxRepository.MarkEventUnpublished")
	defer span.End()

	query := `
		UPDATE outbox
		SET published_at = NULL, last_error = NULL
		WHERE id = $1
	`

	_, err := tx.Exec(ctx, query, eventID)

	if err != nil {
		span.RecordError(err)
	}

	return err
}

func (r *outboxRepo) SaveOutboxEvent(ctx context.Context, tx pgx.Tx, event *domain.OutboxEvent) error {
	ctx, span := r.tracer.Start(ctx, "OutboxRepository.SaveOutboxEvent")
	defer span.End()

	span.SetAttributes(
		attribute.String("aggregate_id", event.AggregateID),
		attribute.String("aggregate_type", event.AggregateType),
	)

	query := `
		INSERT INTO outbox (aggregate_type, aggregate_id, event_type, payload, topic)
		VALUES ($1, $2, $3, $4, $5)
	`

	_, err := tx.Exec(
		ctx,
		query,
		event.AggregateType,
		event.AggregateID,
		event.EventType,
		event.Payload,
		event.Topic,
	)

	if err != nil {
		span.RecordError(err)
	}

	return err
}

func (r *outboxRepo) GetUnpublishedEvents(ctx context.Context, tx pgx.Tx, batchSize int) ([]*domain.OutboxEvent, error) {
	ctx, span := r.tracer.Start(ctx, "OutboxRepository.GetUnpublishedEvents")
	defer span.End()

	span.SetAttributes(
		attribute.Int("batch_size", batchSize),
	)

	query := `
		SELECT id, aggregate_type, aggregate_id, event_type, payload, headers, created_at, topic
		FROM outbox
		WHERE published_at IS NULL AND attempts < 10
		ORDER BY created_at ASC
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`

	rows, err := tx.Query(
		ctx,
		query,
		batchSize,
	)
	if err != nil {
		span.RecordError(err)

		return nil, fmt.Errorf("failed to query unpublished events: %w", err)
	}
	defer rows.Close()

	var events []*domain.OutboxEvent
	for rows.Next() {
		var e domain.OutboxEvent
		if err := rows.Scan(
			&e.Id,
			&e.AggregateType,
			&e.AggregateID,
			&e.EventType,
			&e.Payload,
			&e.Headers,
			&e.CreatedAt,
			&e.Topic,
		); err != nil {
			span.RecordError(err)

			return nil, fmt.Errorf("error scanning event: %w", err)
		}

		events = append(events, &e)
	}

	span.SetAttributes(
		attribute.Int("result_count", len(events)),
	)

	return events, nil
}
