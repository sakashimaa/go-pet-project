-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS processed_events (
    event_id BIGINT PRIMARY KEY,
    processed_at TIMESTAMP DEFAULT NOW()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS processed_events;
-- +goose StatementEnd
