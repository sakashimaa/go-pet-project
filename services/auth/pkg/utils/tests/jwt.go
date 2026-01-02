package tests

import (
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/sakashimaa/go-pet-project/pkg/utils"
	"github.com/stretchr/testify/require"
)

func ValidateTokens(t *testing.T, access, refresh string) {
	parts := strings.Split(access, ".")
	require.Len(t, parts, 3)

	parts = strings.Split(refresh, ".")
	require.Len(t, parts, 3)

	require.NotEqual(t, access, refresh)

	token, err := jwt.Parse(access, func(token *jwt.Token) (interface{}, error) {
		return []byte(utils.ParseWithFallback("ACCESS_SECRET", "")), nil
	})

	require.NoError(t, err)
	require.True(t, token.Valid)

	token, err = jwt.Parse(refresh, func(token *jwt.Token) (any, error) {
		return []byte(utils.ParseWithFallback("REFRESH_SECRET", "")), nil
	})

	require.NoError(t, err)
	require.True(t, token.Valid)
}
