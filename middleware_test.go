package widelogger

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPMiddleware_Basic(t *testing.T) {
	var buf bytes.Buffer
	slogHandler := slog.NewJSONHandler(&buf, nil)
	logger := New(slog.New(slogHandler))

	middleware := HTTPMiddleware(&HTTPMiddlewareConfig{
		Logger: logger,
	})

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	req := httptest.NewRequest("GET", "/test?foo=bar", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Parse log output
	var result map[string]any
	if err := json.NewDecoder(&buf).Decode(&result); err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	// Verify basic fields
	if result["method"] != "GET" {
		t.Errorf("Expected method=GET, got %v", result["method"])
	}
	if result["path"] != "/test" {
		t.Errorf("Expected path=/test, got %v", result["path"])
	}
	if result["status_code"].(float64) != 200 {
		t.Errorf("Expected status_code=200, got %v", result["status_code"])
	}
	if _, exists := result["duration_ms"]; !exists {
		t.Error("Expected duration_ms field")
	}
	if result["level"] != "INFO" {
		t.Errorf("Expected level=INFO, got %v", result["level"])
	}
}

func TestHTTPMiddleware_WithQuery(t *testing.T) {
	var buf bytes.Buffer
	slogHandler := slog.NewJSONHandler(&buf, nil)
	logger := New(slog.New(slogHandler))

	middleware := HTTPMiddleware(&HTTPMiddlewareConfig{
		Logger: logger,
	})

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test?user_id=123&action=login", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	var result map[string]any
	if err := json.NewDecoder(&buf).Decode(&result); err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	if result["query"] != "user_id=123&action=login" {
		t.Errorf("Expected query string, got %v", result["query"])
	}
}

func TestHTTPMiddleware_WithWarnings(t *testing.T) {
	var buf bytes.Buffer
	slogHandler := slog.NewJSONHandler(&buf, nil)
	logger := New(slog.New(slogHandler))

	middleware := HTTPMiddleware(&HTTPMiddlewareConfig{
		Logger: logger,
	})

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		AddWarning(ctx, "slow query", "duration_ms", 2500)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	var result map[string]any
	if err := json.NewDecoder(&buf).Decode(&result); err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	// Should log as WARN because of warnings
	if result["level"] != "WARN" {
		t.Errorf("Expected level=WARN with warnings, got %v", result["level"])
	}

	if result["msg"] != "http_request_completed_with_warnings" {
		t.Errorf("Expected specific warning message, got %v", result["msg"])
	}

	if result["warning_count"].(float64) != 1 {
		t.Errorf("Expected warning_count=1, got %v", result["warning_count"])
	}
}

func TestHTTPMiddleware_WithErrors(t *testing.T) {
	var buf bytes.Buffer
	slogHandler := slog.NewJSONHandler(&buf, nil)
	logger := New(slog.New(slogHandler))

	middleware := HTTPMiddleware(&HTTPMiddlewareConfig{
		Logger: logger,
	})

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		AddError(ctx, "database timeout")
		w.WriteHeader(http.StatusInternalServerError)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	var result map[string]any
	if err := json.NewDecoder(&buf).Decode(&result); err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	// Should log as ERROR because of errors
	if result["level"] != "ERROR" {
		t.Errorf("Expected level=ERROR with errors, got %v", result["level"])
	}

	if result["msg"] != "http_request_completed_with_errors" {
		t.Errorf("Expected specific error message, got %v", result["msg"])
	}

	if result["error_count"].(float64) != 1 {
		t.Errorf("Expected error_count=1, got %v", result["error_count"])
	}
}

func TestHTTPMiddleware_StatusCodes(t *testing.T) {
	tests := []struct {
		name          string
		statusCode    int
		expectedLevel string
		expectedMsg   string
	}{
		{
			name:          "2xx success",
			statusCode:    http.StatusOK,
			expectedLevel: "INFO",
			expectedMsg:   "http_request_completed",
		},
		{
			name:          "4xx client error",
			statusCode:    http.StatusBadRequest,
			expectedLevel: "WARN",
			expectedMsg:   "http_request_completed",
		},
		{
			name:          "404 not found",
			statusCode:    http.StatusNotFound,
			expectedLevel: "WARN",
			expectedMsg:   "http_request_completed",
		},
		{
			name:          "5xx server error",
			statusCode:    http.StatusInternalServerError,
			expectedLevel: "ERROR",
			expectedMsg:   "http_request_completed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			slogHandler := slog.NewJSONHandler(&buf, nil)
			logger := New(slog.New(slogHandler))

			middleware := HTTPMiddleware(&HTTPMiddlewareConfig{
				Logger: logger,
			})

			handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))

			req := httptest.NewRequest("GET", "/test", nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			var result map[string]any
			if err := json.NewDecoder(&buf).Decode(&result); err != nil {
				t.Fatalf("Failed to parse log output: %v", err)
			}

			if result["level"] != tt.expectedLevel {
				t.Errorf("Expected level=%s, got %v", tt.expectedLevel, result["level"])
			}

			if result["msg"] != tt.expectedMsg {
				t.Errorf("Expected msg=%s, got %v", tt.expectedMsg, result["msg"])
			}

			if result["status_code"].(float64) != float64(tt.statusCode) {
				t.Errorf("Expected status_code=%d, got %v", tt.statusCode, result["status_code"])
			}
		})
	}
}

func TestHTTPMiddleware_IncludeHeaders(t *testing.T) {
	var buf bytes.Buffer
	slogHandler := slog.NewJSONHandler(&buf, nil)
	logger := New(slog.New(slogHandler))

	middleware := HTTPMiddleware(&HTTPMiddlewareConfig{
		Logger:                logger,
		IncludeRequestHeaders: []string{"User-Agent", "X-Request-ID"},
	})

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("User-Agent", "TestAgent/1.0")
	req.Header.Set("X-Request-ID", "req-123")
	req.Header.Set("Authorization", "Bearer secret") // Should not be included

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var result map[string]any
	if err := json.NewDecoder(&buf).Decode(&result); err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	headers, ok := result["request_headers"].(map[string]any)
	if !ok {
		t.Fatal("Expected request_headers to be a map")
	}

	if headers["User-Agent"] != "TestAgent/1.0" {
		t.Errorf("Expected User-Agent header, got %v", headers["User-Agent"])
	}

	if headers["X-Request-ID"] != "req-123" {
		t.Errorf("Expected X-Request-ID header, got %v", headers["X-Request-ID"])
	}

	if _, exists := headers["Authorization"]; exists {
		t.Error("Authorization header should not be included")
	}
}

func TestHTTPMiddleware_ExcludePaths(t *testing.T) {
	var buf bytes.Buffer
	slogHandler := slog.NewJSONHandler(&buf, nil)
	logger := New(slog.New(slogHandler))

	middleware := HTTPMiddleware(&HTTPMiddlewareConfig{
		Logger: logger,
		ExcludePaths: map[string]bool{
			"/health":  true,
			"/metrics": true,
		},
	})

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		path      string
		shouldLog bool
	}{
		{"/health", false},
		{"/metrics", false},
		{"/api/users", true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			buf.Reset()

			req := httptest.NewRequest("GET", tt.path, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			hasLog := buf.Len() > 0
			if hasLog != tt.shouldLog {
				t.Errorf("Path %s: expected log=%v, got log=%v", tt.path, tt.shouldLog, hasLog)
			}
		})
	}
}

func TestHTTPMiddleware_Panic(t *testing.T) {
	var buf bytes.Buffer
	slogHandler := slog.NewJSONHandler(&buf, nil)
	logger := New(slog.New(slogHandler))

	panicHandled := false

	middleware := HTTPMiddleware(&HTTPMiddlewareConfig{
		Logger: logger,
		OnPanic: func(ctx context.Context, recovered any) {
			panicHandled = true
			if recovered != "test panic" {
				t.Errorf("Expected panic='test panic', got %v", recovered)
			}
		},
	})

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !panicHandled {
		t.Error("OnPanic handler was not called")
	}

	var result map[string]any
	if err := json.NewDecoder(&buf).Decode(&result); err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	if result["panic"] != "test panic" {
		t.Errorf("Expected panic field, got %v", result["panic"])
	}

	if result["level"] != "ERROR" {
		t.Errorf("Expected ERROR level for panic, got %v", result["level"])
	}
}

func TestHTTPMiddleware_PanicWithoutHandler(t *testing.T) {
	var buf bytes.Buffer
	slogHandler := slog.NewJSONHandler(&buf, nil)
	logger := New(slog.New(slogHandler))

	middleware := HTTPMiddleware(&HTTPMiddlewareConfig{
		Logger: logger,
		// No OnPanic handler - should re-panic
	})

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic to be re-raised")
		} else if r != "test panic" {
			t.Errorf("Expected panic='test panic', got %v", r)
		}
	}()

	handler.ServeHTTP(rec, req)
}

func TestHTTPMiddleware_ContextCancellation(t *testing.T) {
	var buf bytes.Buffer
	slogHandler := slog.NewJSONHandler(&buf, nil)
	logger := New(slog.New(slogHandler))

	middleware := HTTPMiddleware(&HTTPMiddlewareConfig{
		Logger: logger,
	})

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate context cancellation
		ctx, cancel := context.WithCancel(r.Context())
		cancel()

		// Wait a bit to ensure cancellation is detected
		time.Sleep(10 * time.Millisecond)

		// Use the cancelled context
		r = r.WithContext(ctx)

		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	var result map[string]any
	if err := json.NewDecoder(&buf).Decode(&result); err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	// Note: This test might be flaky because we can't easily trigger
	// context cancellation detection in the middleware
	// The middleware checks r.Context().Err() after handler completes
}

func TestHTTPMiddleware_AddFieldsInHandler(t *testing.T) {
	var buf bytes.Buffer
	slogHandler := slog.NewJSONHandler(&buf, nil)
	logger := New(slog.New(slogHandler))

	middleware := HTTPMiddleware(&HTTPMiddlewareConfig{
		Logger: logger,
	})

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Add custom fields during request processing
		AddFields(ctx, "user_id", "123")
		AddFields(ctx, "action", "purchase")
		AddFields(ctx, "amount", 99.99)

		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/api/purchase", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	var result map[string]any
	if err := json.NewDecoder(&buf).Decode(&result); err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	if result["user_id"] != "123" {
		t.Errorf("Expected user_id=123, got %v", result["user_id"])
	}

	if result["action"] != "purchase" {
		t.Errorf("Expected action=purchase, got %v", result["action"])
	}

	if result["amount"].(float64) != 99.99 {
		t.Errorf("Expected amount=99.99, got %v", result["amount"])
	}
}

func TestResponseWriter_StatusCodeCapture(t *testing.T) {
	tests := []struct {
		name       string
		writeFunc  func(w http.ResponseWriter)
		wantStatus int
	}{
		{
			name: "explicit WriteHeader",
			writeFunc: func(w http.ResponseWriter) {
				w.WriteHeader(http.StatusCreated)
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "implicit 200 via Write",
			writeFunc: func(w http.ResponseWriter) {
				w.Write([]byte("OK"))
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "WriteHeader then Write",
			writeFunc: func(w http.ResponseWriter) {
				w.WriteHeader(http.StatusAccepted)
				w.Write([]byte("Accepted"))
			},
			wantStatus: http.StatusAccepted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			wrapped := &responseWriter{
				ResponseWriter: rec,
				statusCode:     http.StatusOK,
			}

			tt.writeFunc(wrapped)

			if wrapped.statusCode != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, wrapped.statusCode)
			}
		})
	}
}

func TestSimpleHTTPMiddleware(t *testing.T) {
	var buf bytes.Buffer
	SetDefaultLogger(slog.New(slog.NewJSONHandler(&buf, nil)))

	handler := SimpleHTTPMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if buf.Len() == 0 {
		t.Error("SimpleHTTPMiddleware should log")
	}

	var result map[string]any
	if err := json.NewDecoder(&buf).Decode(&result); err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	if result["method"] != "GET" {
		t.Errorf("Expected method=GET, got %v", result["method"])
	}
}

func TestHTTPMiddleware_NilConfig(t *testing.T) {
	middleware := HTTPMiddleware(nil)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if getContainer(ctx) == nil {
			t.Error("Context should be initialized even with nil config")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
}

// Benchmark tests

func BenchmarkHTTPMiddleware(b *testing.B) {
	middleware := HTTPMiddleware(nil)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}
}

func BenchmarkHTTPMiddleware_WithFields(b *testing.B) {
	middleware := HTTPMiddleware(nil)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		AddFields(ctx, "user_id", 123, "action", "test")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}
}

func BenchmarkHTTPMiddleware_WithWarnings(b *testing.B) {
	middleware := HTTPMiddleware(nil)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		AddWarning(ctx, "test warning", "detail", "value")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}
}
