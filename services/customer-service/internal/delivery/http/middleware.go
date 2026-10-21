package http

import (
	"net/http"

	"voucher-flash-sale/pkg/contract"
)

func RequireAPIKey(expectedKey string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("x-api-key")
		if apiKey == "" || apiKey != expectedKey {
			contract.WriteError(
				w,
				http.StatusUnauthorized,
				"UNAUTHORIZED",
				"Authentication failed",
				"Invalid or missing x-api-key header",
				nil,
				nil,
			)
			return
		}
		next(w, r)
	}
}
