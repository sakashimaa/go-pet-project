package repository

import "errors"

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrUserAlreadyExists = errors.New("user already exists")
	ErrSessionNotFound   = errors.New("session not found")
	ErrInvalidToken      = errors.New("invalid token")
)
