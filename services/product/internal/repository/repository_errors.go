package repository

import "errors"

var (
	ErrProductAlreadyExists = errors.New("product already exists")
	ErrInsufficientStock    = errors.New("insufficient stock")
	ErrProductNotFound      = errors.New("product not found")
	ErrInvalidInput         = errors.New("invalid input")
)
