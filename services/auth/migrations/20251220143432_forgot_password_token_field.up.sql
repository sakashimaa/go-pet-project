-- +goose Up
-- +goose StatementBegin
ALTER TABLE users
ADD COLUMN forgot_password_token VARCHAR(255);
CREATE INDEX forgot_password_idx ON users(forgot_password_token);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- ALTER TABLE users
-- DROP COLUMN forgot_password_token;
-- DROP INDEX forgot_password_idx;
-- +goose StatementEnd
