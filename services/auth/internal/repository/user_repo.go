package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sakashimaa/go-pet-project/auth/internal/domain"
	"github.com/sakashimaa/go-pet-project/pkg/mylogger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type UserRepository interface {
	Create(ctx context.Context, tx pgx.Tx, user *domain.User) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	GetByID(ctx context.Context, id int64) (*domain.User, error)
	SaveSessionToDB(ctx context.Context, session *domain.RefreshSession) error
	FindSessionByToken(ctx context.Context, token string) (*domain.RefreshSession, error)
	DeleteSessionByID(ctx context.Context, id int64) error
	DeleteSessionByToken(ctx context.Context, token string) error
	VerifyUser(ctx context.Context, token string) error
	SetForgotPasswordToken(ctx context.Context, tx pgx.Tx, email string, token string) error
	ResetPassword(ctx context.Context, tx pgx.Tx, token string, newPassword string) (string, error)
	FindUserByID(ctx context.Context, id int64) (*domain.User, error)
}

type verifyUserRepository struct {
	pool   *pgxpool.Pool
	tracer trace.Tracer
	logger *zap.Logger
}

func NewUserRepository(pool *pgxpool.Pool, logger *zap.Logger) UserRepository {
	return &verifyUserRepository{
		pool:   pool,
		logger: logger,
		tracer: otel.Tracer("repository/user_repo"),
	}
}

func (r *verifyUserRepository) FindUserByID(ctx context.Context, id int64) (*domain.User, error) {
	ctx, span := r.tracer.Start(ctx, "UserRepository.FindUserByID")
	defer span.End()

	span.SetAttributes(
		attribute.Int64("id", id),
	)

	query := `
		SELECT id, is_activated, email
		FROM users
		WHERE id = $1;
	`

	var result domain.User
	if err := r.pool.QueryRow(ctx, query, id).
		Scan(&result.ID, &result.IsActivated, &result.Email); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			span.RecordError(err)

			return nil, ErrUserNotFound
		}

		span.RecordError(err)

		mylogger.Error(
			ctx,
			r.logger,
			"Failed to find user by id",
			zap.Int64("user_id", id),
			zap.Error(err),
		)

		return nil, fmt.Errorf("error finding user: %w", err)
	}

	return &result, nil
}

func (r *verifyUserRepository) ResetPassword(ctx context.Context, tx pgx.Tx, token string, newPassword string) (string, error) {
	ctx, span := r.tracer.Start(ctx, "UserRepository.ResetPassword")
	defer span.End()

	query := `
		UPDATE users
		SET password_hash = $1, forgot_password_token = ''
		WHERE forgot_password_token = $2
		RETURNING email;
	`

	var email string

	err := tx.QueryRow(ctx, query, newPassword, token).
		Scan(&email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			span.RecordError(err)

			return "", ErrUserNotFound
		}

		span.RecordError(err)

		mylogger.Error(
			ctx,
			r.logger,
			"Failed to reset password",
			zap.Error(err),
		)

		return "", fmt.Errorf("error resetting user password: %w", err)
	}

	return email, nil
}

func (r *verifyUserRepository) SetForgotPasswordToken(ctx context.Context, tx pgx.Tx, email string, token string) error {
	ctx, span := r.tracer.Start(ctx, "UserRepository.SetForgotPasswordToken")
	defer span.End()

	span.SetAttributes(
		attribute.String("email", email),
	)

	query := `
		UPDATE users
		SET forgot_password_token = $1
		WHERE email = $2
		RETURNING id;
 	`

	var id int64

	err := tx.QueryRow(ctx, query, token, email).
		Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			span.RecordError(err)

			return ErrUserNotFound
		}

		span.RecordError(err)

		mylogger.Error(
			ctx,
			r.logger,
			"Failed to set forgot password token",
			zap.String("email", email),
			zap.Error(err),
		)

		return fmt.Errorf("error setting token for user: %w", err)
	}

	return nil
}

func (r *verifyUserRepository) VerifyUser(ctx context.Context, token string) error {
	ctx, span := r.tracer.Start(ctx, "UserRepository.VerifyUser")
	defer span.End()

	query := `
		UPDATE users
		SET is_activated = true, activation_token = ''
		WHERE activation_token = $1
		RETURNING id;
    `

	var id int64

	err := r.pool.QueryRow(ctx, query, token).Scan(&id)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			span.RecordError(err)

			return ErrInvalidToken
		}

		span.RecordError(err)

		mylogger.Error(
			ctx,
			r.logger,
			"Error verifying user",
			zap.Error(err),
		)

		return fmt.Errorf("error verifying user: %w", err)
	}

	return nil
}

func (r *verifyUserRepository) DeleteSessionByToken(ctx context.Context, token string) error {
	ctx, span := r.tracer.Start(ctx, "UserRepository.DeleteSessionByToken")
	defer span.End()

	query := `
		DELETE FROM refresh_sessions
		WHERE token = $1;
	`

	ct, err := r.pool.Exec(ctx, query, token)

	if err != nil {
		span.RecordError(err)

		mylogger.Error(
			ctx,
			r.logger,
			"Failed to delete session by token",
			zap.Error(err),
		)

		return fmt.Errorf("error deleting session: %w", err)
	}

	if ct.RowsAffected() == 0 {
		return ErrSessionNotFound
	}

	return nil
}

func (r *verifyUserRepository) DeleteSessionByID(ctx context.Context, id int64) error {
	ctx, span := r.tracer.Start(ctx, "UserRepository.DeleteSessionByID")
	defer span.End()

	span.SetAttributes(
		attribute.Int64("id", id),
	)

	query := `
		DELETE FROM refresh_sessions
		WHERE id = $1;
	`

	ct, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		span.RecordError(err)

		mylogger.Error(
			ctx,
			r.logger,
			"Failed to delete session by id",
			zap.Int64("id", id),
			zap.Error(err),
		)

		return fmt.Errorf("error deleting session: %w", err)
	}

	if ct.RowsAffected() == 0 {
		return ErrSessionNotFound
	}

	return nil
}

func (r *verifyUserRepository) FindSessionByToken(ctx context.Context, token string) (*domain.RefreshSession, error) {
	ctx, span := r.tracer.Start(ctx, "UserRepository.FindSessionByToken")
	defer span.End()

	query := `
		SELECT id, user_id, token, expires_at, created_at
		FROM refresh_sessions
		WHERE token = $1;
	`

	var result domain.RefreshSession
	if err := r.pool.QueryRow(ctx, query, token).
		Scan(&result.ID, &result.UserID, &result.Token, &result.ExpiresAt, &result.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			span.RecordError(err)

			return nil, ErrSessionNotFound
		}

		span.RecordError(err)
		return nil, fmt.Errorf("error getting session: %w", err)
	}

	return &result, nil
}

func (r *verifyUserRepository) Create(ctx context.Context, tx pgx.Tx, user *domain.User) (*domain.User, error) {
	ctx, span := r.tracer.Start(ctx, "UserRepository.Create")
	defer span.End()

	query := `
		INSERT INTO users (email, password_hash, activation_token)
		VALUES ($1, $2, $3)
		RETURNING id, created_at, updated_at;
	`

	span.SetAttributes(
		attribute.String("user.email", user.Email),
	)

	err := tx.QueryRow(ctx, query, user.Email, user.Password, user.ActivationToken).
		Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		span.RecordError(err)

		var pgError *pgconn.PgError

		if errors.As(err, &pgError) {
			if pgError.Code == "23505" {
				mylogger.Warn(
					ctx,
					r.logger,
					"User already exists",
					zap.String("email", user.Email),
				)

				return nil, ErrUserAlreadyExists
			}
		}

		mylogger.Error(
			ctx,
			r.logger,
			"Failed to Create user",
			zap.String("email", user.Email),
			zap.Error(err),
		)

		return nil, fmt.Errorf("error creating user: %w", err)
	}

	return user, nil
}

func (r *verifyUserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	ctx, span := r.tracer.Start(ctx, "UserRepository.GetByEmail")
	defer span.End()

	span.SetAttributes(
		attribute.String("email", email),
	)

	query := `
		SELECT id, email, is_activated, password_hash, created_at, updated_at
		FROM users
		WHERE email = $1;
	`

	var user domain.User
	if err := r.pool.QueryRow(ctx, query, email).
		Scan(&user.ID, &user.Email, &user.IsActivated, &user.Password, &user.CreatedAt, &user.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			span.RecordError(err)
			return nil, ErrUserNotFound
		}

		span.RecordError(err)

		mylogger.Error(
			ctx,
			r.logger,
			"Failed to Get by email",
			zap.String("email", email),
			zap.Error(err),
		)

		return nil, fmt.Errorf("error getting user: %w", err)
	}

	return &user, nil
}

func (r *verifyUserRepository) GetByID(ctx context.Context, id int64) (*domain.User, error) {
	ctx, span := r.tracer.Start(ctx, "UserRepository.GetByID")
	defer span.End()

	span.SetAttributes(
		attribute.Int64("id", id),
	)

	query := `
		SELECT id, email, is_activated
		FROM users
		WHERE id = $1;
 	`

	var user domain.User
	if err := r.pool.QueryRow(ctx, query, id).
		Scan(&user.ID, &user.Email, &user.IsActivated); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			span.RecordError(err)
			return nil, ErrUserNotFound
		}

		span.RecordError(err)

		mylogger.Error(
			ctx,
			r.logger,
			"Failed to Get by ID",
			zap.Int64("id", id),
			zap.Error(err),
		)

		return nil, fmt.Errorf("error getting user: %w", err)
	}

	return &user, nil
}

func (r *verifyUserRepository) SaveSessionToDB(ctx context.Context, session *domain.RefreshSession) error {
	ctx, span := r.tracer.Start(ctx, "UserRepository.SaveSessionToDB")
	defer span.End()

	span.SetAttributes(
		attribute.Int64("id", session.ID),
		attribute.Int64("user_id", session.UserID),
	)

	query := `
		INSERT INTO refresh_sessions (user_id, token, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, created_at;
 	`

	if err := r.pool.QueryRow(ctx, query, session.UserID, session.Token, session.ExpiresAt).
		Scan(&session.ID, &session.CreatedAt); err != nil {
		span.RecordError(err)

		mylogger.Error(
			ctx,
			r.logger,
			"Failed to save session to db",
			zap.Int64("user_id", session.UserID),
			zap.Error(err),
		)

		return fmt.Errorf("error creating refresh session: %w", err)
	}

	return nil
}
