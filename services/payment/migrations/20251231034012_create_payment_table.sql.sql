-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS payments (
    id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'PENDING',
    amount BIGINT NOT NULL,
    transaction_id VARCHAR(255) NOT NULL,

    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
CREATE INDEX idx_payments_order_id ON payments(order_id);
CREATE UNIQUE INDEX idx_payments_order_id_unique ON payments(order_id) WHERE status = 'SUCCEEDED';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS payments;
DROP INDEX IF EXISTS idx_payments_order_id;
DROP INDEX IF EXISTS idx_payments_order_id_unique;
-- +goose StatementEnd
