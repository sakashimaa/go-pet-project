package domain

import "time"

type Product struct {
	ID            int64     `db:"id"`
	Name          string    `db:"name"`
	Description   string    `db:"description"`
	Price         int64     `db:"price"`
	StockQuantity int64     `db:"stock_quantity"`
	ImageUrl      string    `db:"image_url"`
	Category      string    `db:"category"`
	CreatedAt     time.Time `db:"created_at"`
	UpdatedAt     time.Time `db:"updated_at"`
	DeletedAt     time.Time `db:"deleted_at" json:"-"`
}

type UpdateProductInput struct {
	Name          *string `json:"name"`
	Description   *string `json:"description"`
	Price         *int64  `json:"price"`
	StockQuantity *int64  `json:"stock_quantity"`
	ImageUrl      *string `json:"image_url"`
	Category      *string `json:"category"`
}
