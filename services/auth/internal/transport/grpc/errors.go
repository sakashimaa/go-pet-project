package grpc

import (
	"errors"

	"github.com/sakashimaa/go-pet-project/auth/internal/repository"
	"google.golang.org/grpc/codes"
)

func mapErrorCode(err error) codes.Code {
	switch {
	case errors.Is(err, repository.ErrUserNotFound):
		return codes.NotFound
	case errors.Is(err, repository.ErrSessionNotFound):
		return codes.NotFound
	case errors.Is(err, repository.ErrUserAlreadyExists):
		return codes.FailedPrecondition
	case errors.Is(err, repository.ErrInvalidToken):
		return codes.InvalidArgument
	default:
		return codes.Internal
	}
}
