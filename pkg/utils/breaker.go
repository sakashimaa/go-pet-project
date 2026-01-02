package utils

import "github.com/sony/gobreaker"

func ExecuteWithBreaker[T any](cb *gobreaker.CircuitBreaker, fn func() (T, error)) (T, error) {
	res, err := cb.Execute(func() (interface{}, error) {
		return fn()
	})

	if err != nil {
		return *new(T), err
	}

	return res.(T), nil
}
