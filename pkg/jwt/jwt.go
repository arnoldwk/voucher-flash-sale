// Package jwt provides minimal HS256 JWT parsing and validation.
//
// This is intentionally minimal for a hackathon: it validates signature and expiry
// but does not implement the full JWK/JWKS specification. Configure jwtSecret
// via the JWT_SECRET environment variable.
package jwt

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidToken  = errors.New("invalid token")
	ErrExpiredToken  = errors.New("token expired")
	ErrMissingSecret = errors.New("JWT_SECRET not configured")
)

// Claims represents the payload of a JWT. Only "exp" and "sub" are validated.
type Claims struct {
	Sub string `json:"sub"` // customer/user identifier
	Exp int64  `json:"exp"`
	Iat int64  `json:"iat,omitempty"`
}

// TokenClaims is returned by ValidateToken and maps sub to CustomerID.
type TokenClaims struct {
	CustomerID string `json:"customer_id"`
	Phone      string `json:"phone"`
	Exp        int64  `json:"exp"`
	Iat        int64  `json:"iat"`
}

// Validate parses and validates a JWT token signed with HS256.
// It returns the claims if the signature is valid and the token is not expired.
func Validate(token string) (*Claims, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return nil, ErrMissingSecret
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}

	// Verify signature
	sigInput := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(sigInput))
	expectedSig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	// Also try RawStdEncoding for some JWT libraries
	var valid bool
	if expectedSig == parts[2] {
		valid = true
	} else {
		stdSig := base64.RawStdEncoding.EncodeToString(mac.Sum(nil))
		valid = stdSig == parts[2]
		if !valid {
			// Try standard base64 with padding
			std2 := base64.StdEncoding.EncodeToString(mac.Sum(nil))
			valid = std2 == parts[2]
		}
	}

	if !valid {
		return nil, ErrInvalidToken
	}

	// Decode payload
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		payload, err = base64.StdEncoding.DecodeString(parts[1])
		if err != nil {
			return nil, fmt.Errorf("%w: decode payload: %w", ErrInvalidToken, err)
		}
	}

	var claims Claims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("%w: parse claims: %w", ErrInvalidToken, err)
	}

	// Check expiry
	if claims.Exp > 0 && time.Now().Unix() >= claims.Exp {
		return nil, ErrExpiredToken
	}

	return &claims, nil
}

// Subject returns the "sub" claim from a validated token.
// Returns empty string if token is invalid.
func Subject(token string) string {
	claims, err := Validate(token)
	if err != nil {
		return ""
	}
	return claims.Sub
}

// ValidateToken parses and validates a JWT using the given secret.
// It returns TokenClaims mapping sub → CustomerID. Used by auth-service tests.
func ValidateToken(token, secret string) (*TokenClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}

	sigInput := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(sigInput))

	var valid bool
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if sig == parts[2] {
		valid = true
	} else if base64.RawStdEncoding.EncodeToString(mac.Sum(nil)) == parts[2] {
		valid = true
	} else if base64.StdEncoding.EncodeToString(mac.Sum(nil)) == parts[2] {
		valid = true
	}
	if !valid {
		return nil, ErrInvalidToken
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		payload, err = base64.StdEncoding.DecodeString(parts[1])
		if err != nil {
			return nil, fmt.Errorf("%w: decode payload: %w", ErrInvalidToken, err)
		}
	}

	var raw struct {
		Sub   string `json:"sub"`
		Phone string `json:"phone"`
		Exp   int64  `json:"exp"`
		Iat   int64  `json:"iat"`
	}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, fmt.Errorf("%w: parse payload: %w", ErrInvalidToken, err)
	}
	if raw.Exp > 0 && time.Now().Unix() >= raw.Exp {
		return nil, ErrExpiredToken
	}
	return &TokenClaims{
		CustomerID: raw.Sub,
		Phone:      raw.Phone,
		Exp:        raw.Exp,
		Iat:        raw.Iat,
	}, nil
}

// TokenPair holds an access and refresh token pair.
type TokenPair struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    int    `json:"expiresIn"`
}

// GenerateTokens creates a signed HS256 JWT access token and a dummy refresh token.
// sub is the customer/user identifier; phone is stored in the payload for convenience;
// secret is the HMAC key; ttl is the token validity duration.
func GenerateTokens(sub, phone string, isCustomer bool, secret string, ttl time.Duration) (*TokenPair, error) {
	now := time.Now().Unix()
	exp := now + int64(ttl.Seconds())

	payload := struct {
		Sub    string `json:"sub"`
		Phone  string `json:"phone"`
		IsCust bool   `json:"is_customer"`
		Exp    int64  `json:"exp"`
		Iat    int64  `json:"iat"`
	}{
		Sub:    sub,
		Phone:  phone,
		IsCust: isCustomer,
		Exp:    exp,
		Iat:    now,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	headerBytes, err := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	if err != nil {
		return nil, err
	}

	headerB64 := base64.RawURLEncoding.EncodeToString(headerBytes)
	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadBytes)

	// Sign header.payload with HMAC-SHA256
	signingInput := headerB64 + "." + payloadB64
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingInput))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	accessToken := signingInput + "." + sig

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: uuid.New().String(), // dummy refresh token
		ExpiresIn:    int(ttl.Seconds()),
	}, nil
}
