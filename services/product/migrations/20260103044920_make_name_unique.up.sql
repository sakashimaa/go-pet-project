-- +goose Up
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_product_name;
ALTER TABLE products
ADD CONSTRAINT products_name_key UNIQUE(name);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- ALTER TABLE products
-- DROP CONSTRAINT IF EXISTS products_name_key;
--
-- CREATE INDEX IF NOT EXISTS idx_product_name ON products(name);
-- +goose StatementEnd
