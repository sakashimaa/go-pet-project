package utils

import (
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

func FormatValidationError(err error) map[string]string {
	errors := make(map[string]string)
	for _, err := range err.(validator.ValidationErrors) {
		field := strings.ToLower(err.Field())

		switch err.Tag() {
		case "required":
			errors[field] = fmt.Sprintf("%s is required", field)
		case "min":
			errors[field] = fmt.Sprintf("%s must be at least %s characters", field, err.Param())
		case "gt":
			errors[field] = fmt.Sprintf("%s must be greater than %s", field, err.Param())
		case "gte":
			errors[field] = fmt.Sprintf("%s must be greater than or equal to %s", field, err.Param())
		case "url":
			errors[field] = fmt.Sprintf("%s must be a valid URL", field)
		default:
			errors[field] = fmt.Sprintf("%s is invalid", field)
		}
	}
	return errors
}
