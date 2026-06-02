package domain

import (
	"time"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

type Product struct {
	ID            int64     `db:"id"`
	Name          string    `db:"name" validate:"required,min=3,max=100"`
	Description   string    `db:"description" validate:"max=1000"`
	Price         int64     `db:"price" validate:"required,gt=0"`
	StockQuantity int64     `db:"stock_quantity" validate:"gte=0"`
	ImageUrl      string    `db:"image_url" validate:"omitempty,url"`
	Category      string    `db:"category" validate:"required"`
	CreatedAt     time.Time `db:"created_at"`
	UpdatedAt     time.Time `db:"updated_at"`
	DeletedAt     time.Time `db:"deleted_at" json:"-"`
}

type UpdateProductInput struct {
	Name          *string `json:"name" validate:"required,min=3,max=100"`
	Description   *string `json:"description" validate:"max=1000"`
	Price         *int64  `json:"price" validate:"required,gt=0"`
	StockQuantity *int64  `json:"stock_quantity" validate:"gte=0"`
	ImageUrl      *string `json:"image_url" validate:"omitempty,url"`
	Category      *string `json:"category"`
}

func (p *Product) Validate() error {
	return validate.Struct(p)
}

func (p *UpdateProductInput) Validate() error {
	return validate.Struct(p)
}
