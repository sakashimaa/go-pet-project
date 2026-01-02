-- +goose Up
-- +goose StatementBegin
ALTER TABLE payments
ADD COLUMN user_id BIGINT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE payments
DROP COLUMN user_id;
-- +goose StatementEnd
