package validator

import (
	"errors"
	"unicode"
)

var (
	ErrPasswordTooShort = errors.New("password must be at least 8 characters long")
	ErrPasswordTooWeak  = errors.New("password must contain at least one digit and one letter")
)

type Validator interface {
	ValidatePassword(password string) error
}

type authValidator struct{}

func NewValidator() Validator {
	return &authValidator{}
}

func (a *authValidator) ValidatePassword(password string) error {
	if len(password) < 8 {
		return ErrPasswordTooShort
	}

	var hasLetter, hasDigit bool
	for _, r := range password {
		if unicode.IsLetter(r) {
			hasLetter = true
		}
		if unicode.IsDigit(r) {
			hasDigit = true
		}
	}

	if !hasLetter || !hasDigit {
		return ErrPasswordTooWeak
	}

	return nil
}
