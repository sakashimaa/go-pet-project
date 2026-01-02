package utils

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sakashimaa/go-pet-project/pkg/mylogger"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

func ProcessWithDeduplication(
	ctx context.Context,
	pool *pgxpool.Pool,
	logger *zap.Logger,
	eventID int64,
	action func() error,
) error {
	span := trace.SpanFromContext(ctx)

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}

	defer func() {
		shutdownCtx := context.WithoutCancel(ctx)

		err = tx.Rollback(shutdownCtx)
		if err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			mylogger.Error(
				shutdownCtx,
				logger,
				"Error rolling back transaction",
				zap.Error(err),
			)
		}
	}()

	query := `
		INSERT INTO processed_events (event_id)
		VALUES ($1)
	`

	_, err = tx.Exec(ctx, query, eventID)
	if err != nil {
		var pgError *pgconn.PgError
		if errors.As(err, &pgError) && pgError.Code == "23505" {
			mylogger.Info(
				ctx,
				logger,
				"Event already processed, skipping",
				zap.Int64("event_id", eventID),
				zap.Error(err),
			)

			return nil
		}

		span.RecordError(err)
		return err
	}

	sent := false
	for i := 0; i < 3; i++ {
		err = action()
		if err == nil {
			sent = true
			break
		}

		if i < 2 {
			time.Sleep(500 * time.Millisecond)
		}
	}

	if !sent {
		mylogger.Error(ctx, logger, "Failed to sent after retries", zap.Error(err))

		return fmt.Errorf("failed to sent: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		span.RecordError(err)

		mylogger.Error(
			ctx,
			logger,
			"Failed to commit transaction",
			zap.Error(err),
		)

		return fmt.Errorf("failed to sent: %w", err)
	}

	return nil
}
