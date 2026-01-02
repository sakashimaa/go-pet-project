package utils

import "os"

func ParseWithFallback(envName string, fallback string) string {
	result := os.Getenv(envName)
	if result == "" {
		result = fallback
	}

	return result
}
