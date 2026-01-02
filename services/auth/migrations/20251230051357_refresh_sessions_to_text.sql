-- +goose Up
-- +goose StatementBegin
ALTER TABLE refresh_sessions
ALTER COLUMN token TYPE TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- ALTER TABLE refresh_sessions
-- ALTER COLUMN token TYPE VARCHAR(255);
-- +goose StatementEnd
