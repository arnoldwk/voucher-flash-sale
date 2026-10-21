package contract

import (
	"encoding/json"
	"net/http"
)

// Response represents the standardized API response envelope defined in login.md
type Response struct {
	Status       bool    `json:"status"`
	Code         string  `json:"code"`
	Message      string  `json:"message"`
	ErrorMessage *string `json:"errorMessage"`
	ErrorData    any     `json:"errorData"`
	Data         any     `json:"data"`
}

// WriteSuccess writes a successful response with status=true
func WriteSuccess(w http.ResponseWriter, httpStatus int, code, message string, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)

	_ = json.NewEncoder(w).Encode(Response{
		Status:       true,
		Code:         code,
		Message:      message,
		ErrorMessage: nil,
		ErrorData:    nil,
		Data:         data,
	})
}

// WriteError writes an error response with status=false
func WriteError(w http.ResponseWriter, httpStatus int, code, message, errorMessage string, errorData any, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)

	var errMsgPtr *string
	if errorMessage != "" {
		errMsgPtr = &errorMessage
	}

	_ = json.NewEncoder(w).Encode(Response{
		Status:       false,
		Code:         code,
		Message:      message,
		ErrorMessage: errMsgPtr,
		ErrorData:    errorData,
		Data:         data,
	})
}
