package utils

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	UserID      int64 `json:"user_id"`
	IsActivated bool  `json:"is_activated"`
	jwt.RegisteredClaims
}

func GenerateTokens(userID int64, isActivated bool) (string, string, error) {
	accessSecret := os.Getenv("ACCESS_SECRET")
	refreshSecret := os.Getenv("REFRESH_SECRET")

	if accessSecret == "" || refreshSecret == "" {
		return "", "", fmt.Errorf("jwt secrets are not found in env")
	}

	accessTokenClaims := Claims{
		UserID:      userID,
		IsActivated: isActivated,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ID:        uuid.New().String(),
		},
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessTokenClaims)
	signedAccessToken, err := accessToken.SignedString([]byte(accessSecret))
	if err != nil {
		return "", "", err
	}

	refreshTokenClaims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(30 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ID:        uuid.New().String(),
		},
	}

	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshTokenClaims)
	signedRefreshToken, err := refreshToken.SignedString([]byte(refreshSecret))
	if err != nil {
		return "", "", err
	}

	return signedAccessToken, signedRefreshToken, nil
}

func ValidateToken(tokenString string, isRefresh bool) (*Claims, error) {
	secretEnv := "ACCESS_SECRET"
	if isRefresh {
		secretEnv = "REFRESH_SECRET"
	}

	secret := os.Getenv(secretEnv)
	if secret == "" {
		return nil, fmt.Errorf("secret not found")
	}

	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(secret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}
