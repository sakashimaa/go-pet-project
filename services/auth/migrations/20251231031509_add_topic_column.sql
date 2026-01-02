-- +goose Up
-- +goose StatementBegin
ALTER TABLE outbox
ADD COLUMN topic VARCHAR(255) NOT NULL DEFAULT 'user_events';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE outbox
DROP COLUMN topic;
-- +goose StatementEnd
