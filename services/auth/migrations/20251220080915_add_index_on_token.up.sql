-- +goose Up
-- +goose StatementBegin
CREATE INDEX token_idx ON refresh_sessions(token);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- DROP INDEX token_idx;
-- +goose StatementEnd
