package repository

import "errors"

var (
	ErrOrderNotFound    = errors.New("order not found")
	ErrOrderAlreadyPaid = errors.New("order already paid")
)
