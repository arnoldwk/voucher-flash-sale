package response

import (
	"encoding/json"
	"net/http"
)

// APIResponse standardizes the JSON response payload.
type APIResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
}

// JSON writes a standardized JSON response.
func JSON(w http.ResponseWriter, statusCode int, success bool, message string, data any, err string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	resp := APIResponse{
		Success: success,
		Message: message,
		Data:    data,
		Error:   err,
	}

	_ = json.NewEncoder(w).Encode(resp)
}

// Success writes a 200/201 success response.
func Success(w http.ResponseWriter, statusCode int, message string, data any) {
	JSON(w, statusCode, true, message, data, "")
}

// Error writes an error response.
func Error(w http.ResponseWriter, statusCode int, errMessage string) {
	JSON(w, statusCode, false, "", nil, errMessage)
}
