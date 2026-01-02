package grpc

import (
	"google.golang.org/grpc/codes"
)

func mapErrorCode(err error) codes.Code {
	switch {
	default:
		return codes.Internal
	}
}
