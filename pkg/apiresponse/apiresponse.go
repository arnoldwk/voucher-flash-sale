// Package apiresponse provides the standard response envelope used by the
// public product APIs.
package apiresponse

import (
	"encoding/json"
	"net/http"
)

// Response codes exposed to clients.
const (
	CodeSuccess             = "SUCCESS"
	CodeInvalidRequest      = "INVALID_REQUEST"
	CodeUnauthorized        = "UNAUTHORIZED"
	CodeForbidden           = "FORBIDDEN"
	CodeNotFound            = "NOT_FOUND"
	CodeInternalError       = "INTERNAL_ERROR"
	CodeCreated             = "CREATED"
	CodeStockUnavailable    = "STOCK_UNAVAILABLE"
	CodeAlreadyEntitled     = "ALREADY_ENTITLED"
	CodeIdempotencyConflict = "IDEMPOTENCY_CONFLICT"
)

// Envelope is the standard response body.
//
// ErrorMessage and ErrorData are always rendered (no omitempty) so clients can
// rely on a stable shape.
type Envelope struct {
	Status       bool    `json:"status"`
	Code         string  `json:"code"`
	Message      string  `json:"message"`
	ErrorMessage *string `json:"errorMessage"`
	ErrorData    any     `json:"errorData"`
	Data         any     `json:"data"`
}

// Success writes a 2xx envelope.
func Success(w http.ResponseWriter, statusCode int, message string, data any) {
	write(w, statusCode, Envelope{
		Status:  true,
		Code:    CodeSuccess,
		Message: message,
		Data:    data,
	})
}

// SuccessWithCode writes a 2xx envelope with a custom response code.
// Used for endpoints such as POST /v1/orders whose contract requires code CREATED.
func SuccessWithCode(w http.ResponseWriter, statusCode int, code, message string, data any) {
	write(w, statusCode, Envelope{
		Status:  true,
		Code:    code,
		Message: message,
		Data:    data,
	})
}

// Error writes a non-2xx envelope.
func Error(w http.ResponseWriter, statusCode int, code, message string) {
	msg := message
	write(w, statusCode, Envelope{
		Status:       false,
		Code:         code,
		Message:      "",
		ErrorMessage: &msg,
		ErrorData:    nil,
		Data:         nil,
	})
}

// ErrorWithData writes a non-2xx envelope with an attached errorData payload.
// Used for structured client-facing errors such as STOCK_UNAVAILABLE where the
// caller needs the offending category.
func ErrorWithData(w http.ResponseWriter, statusCode int, code, message, errorMessage string, errorData any) {
	msg := message
	errMsg := errorMessage
	write(w, statusCode, Envelope{
		Status:       false,
		Code:         code,
		Message:      msg,
		ErrorMessage: &errMsg,
		ErrorData:    errorData,
		Data:         nil,
	})
}

func write(w http.ResponseWriter, statusCode int, env Envelope) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(env)
}
