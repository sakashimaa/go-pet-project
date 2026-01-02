-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS refresh_sessions (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGSERIAL NOT NULL,
    token VARCHAR(255) NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- DROP TABLE IF EXISTS refresh_sessions;
-- +goose StatementEnd
