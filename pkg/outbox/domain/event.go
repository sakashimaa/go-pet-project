package domain

import (
	"encoding/json"
	"time"
)

type OutboxEvent struct {
	Id            int64           `db:"id"`
	AggregateType string          `db:"aggregate_type"`
	AggregateID   string          `db:"aggregate_id"`
	EventType     string          `db:"event_type"`
	Payload       json.RawMessage `db:"payload"`
	Headers       json.RawMessage `db:"headers"`
	CreatedAt     time.Time       `db:"created_at"`
	PublishedAt   *time.Time      `db:"published_at"`
	Attempts      int64           `db:"attempts"`
	LastError     *string         `db:"last_error"`
	Topic         string          `db:"topic"`
}
