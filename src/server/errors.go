package main

import (
	"encoding/json"
	"net/http"
)

// ErrorResponse represents a structured JSON error returned to clients.
type ErrorResponse struct {
	Error string `json:"error"`
}

// writeJSONError writes a structured JSON error response with the given message and HTTP status code.
func writeJSONError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{Error: msg})
}
