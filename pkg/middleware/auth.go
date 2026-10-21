// Package middleware provides reusable HTTP middleware.
package middleware

import (
	"net/http"
	"strings"

	"voucher-flash-sale/pkg/apiresponse"
	"voucher-flash-sale/pkg/jwt"
)

// AuthBearer validates the Authorization: Bearer <token> header.
// On success it stores the token subject in the request context.
func AuthBearer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, err := bearerToken(r)
		if err != nil {
			apiresponse.Error(w, http.StatusUnauthorized, apiresponse.CodeUnauthorized, "missing or malformed Authorization header")
			return
		}

		if _, err := jwt.Validate(token); err != nil {
			apiresponse.Error(w, http.StatusUnauthorized, apiresponse.CodeUnauthorized, "invalid or expired access token")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func bearerToken(r *http.Request) (string, error) {
	header := r.Header.Get("Authorization")
	if header == "" {
		return "", errMissingAuth
	}

	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", errMissingAuth
	}

	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", errMissingAuth
	}
	return token, nil
}

type authError struct{ msg string }

func (e *authError) Error() string { return e.msg }

var errMissingAuth = &authError{msg: "missing bearer token"}
