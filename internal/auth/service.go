// Package auth provides JWT token generation and validation utilities.
// Tokens are signed with HMAC-SHA256 using a secret loaded from configuration.
package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims defines the payload embedded in every JWT token.
// It extends the standard RegisteredClaims with user-specific fields.
type Claims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// GenerateToken creates a signed JWT string for the given user.
// The token expires after the configured number of hours.
func GenerateToken(userID uint, username, secret string, expiresHours int) (string, error) {
	claims := Claims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			// Use UTC time to avoid timezone surprises in distributed environments.
			ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(time.Duration(expiresHours) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}
	return signed, nil
}

// ValidateToken parses and validates a JWT string, returning the embedded claims.
// Returns an error if the token is malformed, expired, or has an invalid signature.
func ValidateToken(tokenString, secret string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (any, error) {
		// Explicitly verify the signing algorithm to prevent "alg:none" attacks.
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}
	return claims, nil
}
