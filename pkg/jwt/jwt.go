package jwt

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken = errors.New("invalid or expired token")
)

type Claims struct {
	CustomerID string `json:"customer_id"`
	Phone      string `json:"phone"`
	IsVerified bool   `json:"is_verified"`
	jwt.RegisteredClaims
}

type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int // in seconds
}

// GenerateTokens creates an HMAC-SHA256 signed JWT access token and a secure random refresh token.
func GenerateTokens(customerID, phone string, isVerified bool, secret string, accessTTL time.Duration) (*TokenPair, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(accessTTL)

	claims := Claims{
		CustomerID: customerID,
		Phone:      phone,
		IsVerified: isVerified,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   customerID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedAccess, err := token.SignedString([]byte(secret))
	if err != nil {
		return nil, fmt.Errorf("failed to sign access token: %w", err)
	}

	// Generate random refresh token
	refreshBytes := make([]byte, 32)
	if _, err := rand.Read(refreshBytes); err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}
	refreshToken := hex.EncodeToString(refreshBytes)

	return &TokenPair{
		AccessToken:  signedAccess,
		RefreshToken: refreshToken,
		ExpiresIn:    int(accessTTL.Seconds()),
	}, nil
}

// ValidateToken parses and validates a signed JWT token.
func ValidateToken(tokenString, secret string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})

	if err != nil {
		return nil, ErrInvalidToken
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, ErrInvalidToken
}
