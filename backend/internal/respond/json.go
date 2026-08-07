package respond

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"urockclimbing.com/backend/internal/middleware"
)

type envelope struct {
	Data      any        `json:"data,omitempty"`
	Error     *ErrorBody `json:"error,omitempty"`
	RequestID string     `json:"request_id"`
}

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func JSON(w http.ResponseWriter, r *http.Request, status int, data any) {
	write(w, status, envelope{Data: data, RequestID: middleware.RequestIDFromContext(r.Context())})
}

func Error(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	write(w, status, envelope{
		Error:     &ErrorBody{Code: code, Message: message},
		RequestID: middleware.RequestIDFromContext(r.Context()),
	})
}

func write(w http.ResponseWriter, status int, body envelope) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Error("encode HTTP response", "error", err)
	}
}
