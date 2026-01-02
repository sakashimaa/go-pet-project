-- +goose Up
-- +goose StatementBegin
ALTER TABLE products
ADD COLUMN deleted_at TIMESTAMPTZ DEFAULT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE products
DROP COLUMN deleted_at;
-- +goose StatementEnd
