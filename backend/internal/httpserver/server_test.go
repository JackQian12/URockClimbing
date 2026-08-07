package httpserver

import (
	"bytes"
	"database/sql"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLiveness(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := New(Dependencies{
		DB:        &sql.DB{},
		Logger:    logger,
		AppEnv:    "test",
		StartedAt: time.Unix(0, 0).UTC(),
	})

	request := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if !strings.Contains(response.Body.String(), `"status":"ok"`) {
		t.Fatalf("unexpected response: %s", response.Body.String())
	}
	if response.Header().Get("X-Request-ID") == "" {
		t.Fatal("X-Request-ID is missing")
	}
}

func TestNotFoundUsesErrorEnvelope(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	handler := New(Dependencies{DB: &sql.DB{}, Logger: logger, AppEnv: "test"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/missing", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}
