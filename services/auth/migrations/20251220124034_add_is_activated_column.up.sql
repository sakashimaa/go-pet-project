-- +goose Up
-- +goose StatementBegin
ALTER TABLE users
ADD COLUMN is_activated bool DEFAULT false;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- ALTER TABLE users
-- DROP COLUMN is_activated;
-- +goose StatementEnd
