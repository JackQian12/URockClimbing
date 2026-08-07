package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"
)

func Recovery(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error("panic recovered",
					"panic", recovered,
					"request_id", RequestIDFromContext(r.Context()),
					"stack", string(debug.Stack()),
				)
				http.Error(w, `{"error":{"code":"INTERNAL_ERROR","message":"服务器内部错误"}}`, http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
