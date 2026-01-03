package repository

import "errors"

var ErrProductNotFound = errors.New("product not found")
var ErrInsufficientStock = errors.New("insufficient stock")
var ErrProductAlreadyExists = errors.New("product already exists")
