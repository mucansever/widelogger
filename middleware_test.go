package widelogger

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMiddleware_Basic(t *testing.T) {
	var buf bytes.Buffer
	slogHandler := slog.NewJSONHandler(&buf, nil)
	logger := New(slog.New(slogHandler))

	middleware := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}), WithLogger(logger))

	req := httptest.NewRequest("GET", "/test?foo=bar", nil)
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	// Parse log output
	var result map[string]any
	if err := json.NewDecoder(&buf).Decode(&result); err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	if result["method"] != "GET" {
		t.Errorf("Expected method=GET, got %v", result["method"])
	}
	if result["path"] != "/test" {
		t.Errorf("Expected path=/test, got %v", result["path"])
	}
	if result["status_code"].(float64) != 200 {
		t.Errorf("Expected status_code=200, got %v", result["status_code"])
	}
}

func TestMiddleware_WithIncludeHeaders(t *testing.T) {
	var buf bytes.Buffer
	slogHandler := slog.NewJSONHandler(&buf, nil)
	logger := New(slog.New(slogHandler))

	middleware := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}),
		WithLogger(logger),
		WithIncludeRequestHeaders("User-Agent"),
	)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("User-Agent", "TestAgent")
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	var result map[string]any
	if err := json.NewDecoder(&buf).Decode(&result); err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	headers, ok := result["request_headers"].(map[string]any)
	if !ok {
		t.Fatal("Expected request_headers to be a map")
	}

	if headers["User-Agent"] != "TestAgent" {
		t.Errorf("Expected User-Agent header, got %v", headers["User-Agent"])
	}
}

func TestMiddleware_ExcludePaths(t *testing.T) {
	var buf bytes.Buffer
	slogHandler := slog.NewJSONHandler(&buf, nil)
	logger := New(slog.New(slogHandler))

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := Middleware(handler,
		WithLogger(logger),
		WithExcludePaths("/health"),
	)

	tests := []struct {
		path      string
		shouldLog bool
	}{
		{"/health", false},
		{"/api/users", true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			buf.Reset()

			req := httptest.NewRequest("GET", tt.path, nil)
			rec := httptest.NewRecorder()

			middleware.ServeHTTP(rec, req)

			hasLog := buf.Len() > 0
			if hasLog != tt.shouldLog {
				t.Errorf("Path %s: expected log=%v, got log=%v", tt.path, tt.shouldLog, hasLog)
			}
		})
	}
}

func TestMiddleware_Panic(t *testing.T) {
	var buf bytes.Buffer
	slogHandler := slog.NewJSONHandler(&buf, nil)
	logger := New(slog.New(slogHandler))

	panicHandled := false

	middleware := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}),
		WithLogger(logger),
		WithPanicHandler(func(ctx context.Context, recovered any) {
			panicHandled = true
			if recovered != "test panic" {
				t.Errorf("Expected panic='test panic', got %v", recovered)
			}
		}),
	)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

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
}

func TestMiddleware_AddFieldsInHandler(t *testing.T) {
	var buf bytes.Buffer
	slogHandler := slog.NewJSONHandler(&buf, nil)
	logger := New(slog.New(slogHandler))

	middleware := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		AddFields(ctx, "user_id", "123")
		w.WriteHeader(http.StatusOK)
	}), WithLogger(logger))

	req := httptest.NewRequest("POST", "/api/purchase", nil)
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	var result map[string]any
	if err := json.NewDecoder(&buf).Decode(&result); err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	if result["user_id"] != "123" {
		t.Errorf("Expected user_id=123, got %v", result["user_id"])
	}
}

func TestMiddleware_Sampling(t *testing.T) {
	var buf bytes.Buffer
	slogHandler := slog.NewJSONHandler(&buf, nil)
	logger := New(slog.New(slogHandler))

	// count lines in buffer (each log is a line)
	countLogs := func() int {
		if buf.Len() == 0 {
			return 0
		}
		return bytes.Count(buf.Bytes(), []byte("\n"))
	}

	t.Run("Rate 0.0", func(t *testing.T) {
		buf.Reset()
		handler := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}), WithLogger(logger), WithSuccessSampling(0.0))

		for i := 0; i < 10; i++ {
			req := httptest.NewRequest("GET", "/", nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
		}

		if count := countLogs(); count != 0 {
			t.Errorf("Rate 0.0: Expected 0 logs for success, got %d", count)
		}

		// error should bypass sampling
		buf.Reset()
		errorHandler := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			AddError(r.Context(), "oops")
			w.WriteHeader(http.StatusOK)
		}), WithLogger(logger), WithSuccessSampling(0.0))

		errorHandler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
		if count := countLogs(); count != 1 {
			t.Errorf("Rate 0.0: Expected 1 log for error, got %d", count)
		}

		// warning should bypass sampling
		buf.Reset()
		warnHandler := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			AddWarning(r.Context(), "hmm")
			w.WriteHeader(http.StatusOK)
		}), WithLogger(logger), WithSuccessSampling(0.0))

		warnHandler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
		if count := countLogs(); count != 1 {
			t.Errorf("Rate 0.0: Expected 1 log for warning, got %d", count)
		}

		// 500 status request should bypass sampling
		buf.Reset()
		failHandler := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}), WithLogger(logger), WithSuccessSampling(0.0))

		failHandler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
		if count := countLogs(); count != 1 {
			t.Errorf("Rate 0.0: Expected 1 log for 500 status, got %d", count)
		}
	})

	t.Run("Rate 1.0", func(t *testing.T) {
		buf.Reset()
		handler := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}), WithLogger(logger), WithSuccessSampling(1.0))

		for i := 0; i < 10; i++ {
			req := httptest.NewRequest("GET", "/", nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
		}

		if count := countLogs(); count != 10 {
			t.Errorf("Rate 1.0: Expected 10 logs, got %d", count)
		}
	})
}

func TestMiddleware_RequestID_Generated(t *testing.T) {
	var buf bytes.Buffer
	slogHandler := slog.NewJSONHandler(&buf, nil)
	logger := New(slog.New(slogHandler))

	middleware := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), WithLogger(logger), WithRequestID())

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	var result map[string]any
	if err := json.NewDecoder(&buf).Decode(&result); err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	requestID, ok := result["request_id"].(string)
	if !ok || requestID == "" {
		t.Error("Expected request_id to be generated")
	}

	// Check UUID format (8-4-4-4-12)
	if len(requestID) != 36 {
		t.Errorf("Expected UUID format, got %s", requestID)
	}
}

func TestMiddleware_RequestID_Extracted(t *testing.T) {
	var buf bytes.Buffer
	slogHandler := slog.NewJSONHandler(&buf, nil)
	logger := New(slog.New(slogHandler))

	middleware := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), WithLogger(logger), WithRequestID())

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-ID", "existing-request-id-123")
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	var result map[string]any
	if err := json.NewDecoder(&buf).Decode(&result); err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	if result["request_id"] != "existing-request-id-123" {
		t.Errorf("Expected existing request ID, got %v", result["request_id"])
	}
}

func TestMiddleware_RequestID_InResponse(t *testing.T) {
	var buf bytes.Buffer
	slogHandler := slog.NewJSONHandler(&buf, nil)
	logger := New(slog.New(slogHandler))

	middleware := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), WithLogger(logger), WithRequestID())

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	responseID := rec.Header().Get("X-Request-ID")
	if responseID == "" {
		t.Error("Expected X-Request-ID in response headers")
	}

	// Verify the response ID matches the logged ID
	var result map[string]any
	if err := json.NewDecoder(&buf).Decode(&result); err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	if result["request_id"] != responseID {
		t.Errorf("Response ID %s does not match logged ID %v", responseID, result["request_id"])
	}
}

func TestMiddleware_RequestID_CustomHeader(t *testing.T) {
	var buf bytes.Buffer
	slogHandler := slog.NewJSONHandler(&buf, nil)
	logger := New(slog.New(slogHandler))

	middleware := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), WithLogger(logger), WithRequestID(&RequestIDConfig{
		HeaderName:          "X-Correlation-ID",
		PropagateToResponse: true,
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Correlation-ID", "custom-correlation-id")
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	var result map[string]any
	if err := json.NewDecoder(&buf).Decode(&result); err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	if result["request_id"] != "custom-correlation-id" {
		t.Errorf("Expected custom-correlation-id, got %v", result["request_id"])
	}

	if rec.Header().Get("X-Correlation-ID") != "custom-correlation-id" {
		t.Error("Expected X-Correlation-ID in response headers")
	}
}

func TestMiddleware_RequestID_CustomGenerator(t *testing.T) {
	var buf bytes.Buffer
	slogHandler := slog.NewJSONHandler(&buf, nil)
	logger := New(slog.New(slogHandler))

	counter := 0
	customGenerator := func() string {
		counter++
		return "custom-id-" + string(rune('0'+counter))
	}

	middleware := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), WithLogger(logger), WithRequestID(&RequestIDConfig{
		Generator:           customGenerator,
		PropagateToResponse: true,
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	var result map[string]any
	if err := json.NewDecoder(&buf).Decode(&result); err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	if result["request_id"] != "custom-id-1" {
		t.Errorf("Expected custom-id-1, got %v", result["request_id"])
	}
}

func TestMiddleware_GetRequestID(t *testing.T) {
	var capturedRequestID string

	middleware := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedRequestID = GetRequestID(r.Context())
		w.WriteHeader(http.StatusOK)
	}), WithRequestID(&RequestIDConfig{
		HeaderName: "X-Request-ID",
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-ID", "test-request-id")
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	if capturedRequestID != "test-request-id" {
		t.Errorf("Expected GetRequestID to return 'test-request-id', got %s", capturedRequestID)
	}
}

func TestMiddleware_RequestID_NoPropagation(t *testing.T) {
	middleware := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), WithRequestID(&RequestIDConfig{
		PropagateToResponse: false,
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	if rec.Header().Get("X-Request-ID") != "" {
		t.Error("Expected no X-Request-ID in response headers when propagation is disabled")
	}
}
