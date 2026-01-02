package grpc

import (
	"errors"

	"github.com/sakashimaa/go-pet-project/product/internal/repository"
	"google.golang.org/grpc/codes"
)

func mapErrorCode(err error) codes.Code {
	switch {
	case errors.Is(err, repository.ErrProductNotFound):
		return codes.NotFound
	case errors.Is(err, repository.ErrInsufficientStock):
		return codes.FailedPrecondition
	default:
		return codes.Internal
	}
}
