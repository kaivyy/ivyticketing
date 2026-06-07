package errors

import (
	"encoding/json"
	"errors"
	"net/http"
)

type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string { return e.Code + ": " + e.Message }

func New(status int, code, message string) *APIError {
	return &APIError{Status: status, Code: code, Message: message}
}

type envelope struct {
	Error body `json:"error"`
}

type body struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"requestId"`
}

// WriteError renders err as the standard JSON envelope. Non-APIErrors are
// masked as a generic 500 (struktur.md §19: never leak internal errors).
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	apiErr := &APIError{Status: http.StatusInternalServerError, Code: "INTERNAL", Message: "internal server error"}
	var ae *APIError
	if errors.As(err, &ae) {
		apiErr = ae
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(apiErr.Status)
	json.NewEncoder(w).Encode(envelope{Error: body{
		Code:      apiErr.Code,
		Message:   apiErr.Message,
		RequestID: r.Header.Get("X-Request-Id"),
	}})
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
