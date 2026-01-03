-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS products (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL,
    price BIGINT NOT NULL CHECK (price >= 0),
    stock_quantity INT NOT NULL DEFAULT 0 NOT NULL CHECK (stock_quantity >= 0),
    image_url TEXT,
    category TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_product_name ON products(name);

CREATE TABLE IF NOT EXISTS outbox (
    id BIGSERIAL PRIMARY KEY,
    aggregate_type TEXT NOT NULL,
    aggregate_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    payload JSONB NOT NULL,
    headers JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    published_at TIMESTAMP,
    attempts INT NOT NULL DEFAULT 0,
    last_error TEXT NULL,
    topic VARCHAR(255) NOT NULL DEFAULT 'user_events'
);
CREATE INDEX IF NOT EXISTS idx_outbox_unpublished
    ON outbox(published_at, created_at)
    WHERE published_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- DROP TABLE IF EXISTS products;
-- DROP INDEX IF EXISTS idx_product_name;
-- DROP TABLE IF EXISTS outbox;
-- DROP INDEX IF EXISTS idx_outbox_unpublished;
-- +goose StatementEnd
