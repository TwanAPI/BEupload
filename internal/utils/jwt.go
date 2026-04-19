package utils

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// CustomClaims extends RegisteredClaims with application-specific fields
type CustomClaims struct {
	Email string `json:"email"`
	jwt.RegisteredClaims
}

// GenerateJWT generates a JWT token with proper claim semantics
func GenerateJWT(userID, email, secret string) (string, error) {
	claims := CustomClaims{
		Email: email,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    "cloud-caddy",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}
