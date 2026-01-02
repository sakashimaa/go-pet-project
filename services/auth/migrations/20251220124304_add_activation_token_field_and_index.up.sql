-- +goose Up
-- +goose StatementBegin
ALTER TABLE users
ADD COLUMN activation_token VARCHAR(255);
CREATE INDEX activation_token_idx ON users(activation_token);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- ALTER TABLE users
-- DROP COLUMN activation_token;
-- DROP INDEX activation_token_idx;
-- +goose StatementEnd
