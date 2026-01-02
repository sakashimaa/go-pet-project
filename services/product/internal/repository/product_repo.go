package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sakashimaa/go-pet-project/pkg/mylogger"
	"github.com/sakashimaa/go-pet-project/product/internal/domain"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type ProductRepository interface {
	Create(ctx context.Context, tx pgx.Tx, product *domain.Product) (int64, error)
	GetByID(ctx context.Context, id int64) (*domain.Product, error)
	List(ctx context.Context, limit, offset int64, search string) ([]domain.Product, int64, error)
	DeleteByID(ctx context.Context, id int64) error
	Update(ctx context.Context, id int64, input *domain.UpdateProductInput) error
	DecreaseStock(ctx context.Context, tx pgx.Tx, id, quantity int64) (int64, error)
	IncreaseStock(ctx context.Context, tx pgx.Tx, id int64, quantity int32) error
}

type productRepo struct {
	pool   *pgxpool.Pool
	tracer trace.Tracer
	logger *zap.Logger
}

func NewProductRepository(pool *pgxpool.Pool, logger *zap.Logger) ProductRepository {
	return &productRepo{
		pool:   pool,
		logger: logger,
		tracer: otel.Tracer("contract/product_repo"),
	}
}

func (r *productRepo) IncreaseStock(ctx context.Context, tx pgx.Tx, id int64, quantity int32) error {
	ctx, span := r.tracer.Start(ctx, "ProductRepository.IncreaseStock")
	defer span.End()

	span.SetAttributes(
		attribute.Int64("id", id),
		attribute.Int("quantity", int(quantity)),
	)

	query := `
		UPDATE products
		SET stock_quantity = stock_quantity + $1, updated_at = NOW()
		WHERE id = $2
	`
	commandTag, err := tx.Exec(ctx, query, quantity, id)
	if err != nil {
		span.RecordError(err)
		mylogger.Warn(ctx, r.logger, "Failed to update stock_quantity", zap.Error(err))

		return err
	}

	if commandTag.RowsAffected() == 0 {
		mylogger.Warn(ctx, r.logger, "Product not found", zap.Int64("product_id", id))
		return ErrProductNotFound
	}

	return nil
}

func (r *productRepo) DecreaseStock(ctx context.Context, tx pgx.Tx, id, quantity int64) (int64, error) {
	ctx, span := r.tracer.Start(ctx, "ProductRepository.DecreaseStock")
	defer span.End()

	span.SetAttributes(
		attribute.Int64("id", id),
		attribute.Int64("quantity", quantity),
	)

	productPriceQuery := `
		SELECT price
		FROM products
		WHERE id = $1
	`

	var price int64
	if err := tx.QueryRow(ctx, productPriceQuery, id).Scan(&price); err != nil {
		mylogger.Error(
			ctx,
			r.logger,
			"Failed to query product",
			zap.Int64("product_id", id),
			zap.Error(err),
		)

		return 0, err
	}

	query := `
		UPDATE products
		SET stock_quantity = stock_quantity - $2, updated_at = NOW()
		WHERE id = $1
			AND stock_quantity >= $2
			AND deleted_at IS NULL;
	`

	commandTag, err := tx.Exec(ctx, query, id, quantity)
	if err != nil {
		span.RecordError(err)

		mylogger.Error(
			ctx,
			r.logger,
			"Error decreasing stock",
			zap.Int64("id", id),
			zap.Int64("quantity", quantity),
		)

		return 0, fmt.Errorf("error decreasing stock for product %d: %w", id, err)
	}

	if commandTag.RowsAffected() == 0 {
		return 0, ErrInsufficientStock
	}

	return price, nil
}

func (r *productRepo) Update(ctx context.Context, id int64, input *domain.UpdateProductInput) error {
	ctx, span := r.tracer.Start(ctx, "ProductRepository.Update")
	defer span.End()

	span.SetAttributes(
		attribute.Int64("id", id),
	)

	query := `UPDATE products SET `
	var args []interface{}
	argId := 1

	var updates []string

	if input.Name != nil {
		updates = append(updates, fmt.Sprintf("name = $%d", argId))
		args = append(args, *input.Name)
		argId++
	}

	if input.Description != nil {
		updates = append(updates, fmt.Sprintf("description = $%d", argId))
		args = append(args, *input.Description)
		argId++
	}

	if input.Price != nil {
		updates = append(updates, fmt.Sprintf("price = $%d", argId))
		args = append(args, *input.Price)
		argId++
	}

	if input.StockQuantity != nil {
		updates = append(updates, fmt.Sprintf("stock_quantity = $%d", argId))
		args = append(args, *input.StockQuantity)
		argId++
	}

	if input.ImageUrl != nil {
		updates = append(updates, fmt.Sprintf("image_url = $%d", argId))
		args = append(args, *input.ImageUrl)
		argId++
	}

	if input.Category != nil {
		updates = append(updates, fmt.Sprintf("category = $%d", argId))
		args = append(args, *input.Category)
		argId++
	}

	if len(updates) == 0 {
		return nil
	}

	updates = append(updates, "updated_at = NOW()")

	query += strings.Join(updates, ", ")
	query += fmt.Sprintf("WHERE id = $%d AND deleted_at IS NULL", argId)
	args = append(args, id)

	commandTag, err := r.pool.Exec(ctx, query, args...)
	if err != nil {
		span.RecordError(err)

		mylogger.Error(
			ctx,
			r.logger,
			"Failed to Update product",
			zap.Int64("id", id),
		)

		return fmt.Errorf("error updating product: %w", err)
	}

	if commandTag.RowsAffected() == 0 {
		return ErrProductNotFound
	}

	return nil
}

func (r *productRepo) DeleteByID(ctx context.Context, id int64) error {
	ctx, span := r.tracer.Start(ctx, "ProductRepository.DeleteByID")
	defer span.End()

	span.SetAttributes(
		attribute.Int64("id", id),
	)

	query := `
		UPDATE products
		SET deleted_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`

	commandTag, err := r.pool.Exec(ctx, query, id)

	if err != nil {
		span.RecordError(err)

		mylogger.Error(
			ctx,
			r.logger,
			"Error deleting product by id",
			zap.Int64("id", id),
			zap.Error(err),
		)

		return fmt.Errorf("error deleting product by id: %v", err)
	}

	if commandTag.RowsAffected() == 0 {
		return ErrProductNotFound
	}

	return nil
}

func (r *productRepo) Create(ctx context.Context, tx pgx.Tx, product *domain.Product) (int64, error) {
	ctx, span := r.tracer.Start(ctx, "ProductRepository.Create")
	defer span.End()

	span.SetAttributes(
		attribute.String("name", product.Name),
	)

	query := `
		INSERT INTO products (name, description, price, stock_quantity, image_url, category)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id;
	`

	err := tx.QueryRow(
		ctx,
		query,
		product.Name,
		product.Description,
		product.Price,
		product.StockQuantity,
		product.ImageUrl,
		product.Category,
	).Scan(&product.ID)
	if err != nil {
		span.RecordError(err)

		mylogger.Error(
			ctx,
			r.logger,
			"Error creating product",
			zap.Error(err),
		)

		return 0, fmt.Errorf("error creating product: %w", err)
	}

	return product.ID, nil
}

func (r *productRepo) GetByID(ctx context.Context, id int64) (*domain.Product, error) {
	ctx, span := r.tracer.Start(ctx, "ProductRepository.GetByID")
	defer span.End()

	span.SetAttributes(
		attribute.Int64("id", id),
	)

	query := `
		SELECT id, name, description, price, stock_quantity,
		image_url, category, created_at, updated_at
		FROM products
		WHERE id = $1 and deleted_at IS NULL;
	`

	var res domain.Product
	if err := r.pool.QueryRow(ctx, query, id).
		Scan(&res.ID, &res.Name, &res.Description, &res.Price,
			&res.StockQuantity, &res.ImageUrl, &res.Category,
			&res.CreatedAt, &res.UpdatedAt,
		); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrProductNotFound
		}

		span.RecordError(err)

		mylogger.Error(
			ctx,
			r.logger,
			"Error get by id",
			zap.Int64("id", id),
			zap.Error(err),
		)

		return nil, fmt.Errorf("error getting product: %w", err)
	}

	return &res, nil
}

func (r *productRepo) List(ctx context.Context, limit, offset int64, search string) ([]domain.Product, int64, error) {
	ctx, span := r.tracer.Start(ctx, "ProductRepository.List")
	defer span.End()

	span.SetAttributes(
		attribute.Int64("limit", limit),
		attribute.Int64("offset", offset),
		attribute.String("search", search),
	)

	var products []domain.Product
	var totalCount int64

	baseQuery := `SELECT id, name, description, price, stock_quantity,
		image_url, category, created_at, updated_at
		FROM products
		WHERE deleted_at IS NULL`
	countQuery := `SELECT COUNT(*) FROM products WHERE deleted_at IS NULL`

	var args []interface{}
	argId := 1

	if search != "" {
		filter := fmt.Sprintf(" AND name ILIKE $%d", argId)
		baseQuery += filter
		countQuery += filter

		args = append(args, "%"+search+"%")
		argId++
	}

	baseQuery += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argId, argId+1)
	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, baseQuery, args...)
	if err != nil {
		span.RecordError(err)

		mylogger.Error(
			ctx,
			r.logger,
			"Error getting products",
			zap.String("search", search),
			zap.Int64("limit", limit),
			zap.Int64("offset", offset),
			zap.Error(err),
		)

		return nil, 0, fmt.Errorf("error selecting products: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var p domain.Product
		err := rows.Scan(
			&p.ID,
			&p.Name,
			&p.Description,
			&p.Price,
			&p.StockQuantity,
			&p.ImageUrl,
			&p.Category,
			&p.CreatedAt,
			&p.UpdatedAt,
		)
		if err != nil {
			span.RecordError(err)

			mylogger.Error(
				ctx,
				r.logger,
				"Failed to scan rows",
				zap.Error(err),
			)

			return nil, 0, fmt.Errorf("error scanning rows: %w", err)
		}
		products = append(products, p)
	}
	if err := rows.Err(); err != nil {
		span.RecordError(err)

		mylogger.Error(
			ctx,
			r.logger,
			"Rows iteration error",
			zap.Error(err),
		)

		return nil, 0, fmt.Errorf("rows iteration error: %w", err)
	}

	var countArgs []interface{}
	if search != "" {
		countArgs = append(countArgs, args[0])
	}

	err = r.pool.QueryRow(ctx, countQuery, countArgs...).Scan(&totalCount)
	if err != nil {
		span.RecordError(err)

		mylogger.Error(
			ctx,
			r.logger,
			"Failed to count products",
			zap.Error(err),
		)

		return nil, 0, fmt.Errorf("failed to count products: %w", err)
	}

	return products, totalCount, nil
}
