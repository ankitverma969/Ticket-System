package response

import (
	"encoding/json"
	"net/http"
)

// JSON writes a standardized JSON response with status code and Content-Type header.
func JSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(data)
}

// ErrorResponse represents a standard error response payload.
type ErrorResponse struct {
	Error string `json:"error"`
}

// Error writes an error response in JSON format.
func Error(w http.ResponseWriter, statusCode int, message string) {
	JSON(w, statusCode, ErrorResponse{Error: message})
}
