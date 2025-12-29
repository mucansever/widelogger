package widelogger

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"testing"
)

func TestNewContext(t *testing.T) {
	ctx := NewContext(context.Background())
	if getContainer(ctx) == nil {
		t.Fatal("NewContext should initialize container")
	}

	container := getContainer(ctx)
	if container.fields == nil {
		t.Error("fields map should be initialized")
	}
	if container.warnings == nil {
		t.Error("warnings slice should be initialized")
	}
	if container.errors == nil {
		t.Error("errors slice should be initialized")
	}
}

func TestAddFields(t *testing.T) {
	tests := []struct {
		name    string
		ctx     context.Context
		args    []any
		wantErr bool
		errType error
	}{
		{
			name:    "valid key-value pairs",
			ctx:     NewContext(context.Background()),
			args:    []any{"key1", "value1", "key2", 123},
			wantErr: false,
		},
		{
			name:    "odd number of arguments",
			ctx:     NewContext(context.Background()),
			args:    []any{"key1", "value1", "key2"},
			wantErr: true,
			errType: ErrOddNumberOfArgs,
		},
		{
			name:    "non-string key",
			ctx:     NewContext(context.Background()),
			args:    []any{123, "value"},
			wantErr: true,
		},
		{
			name:    "uninitialized context",
			ctx:     context.Background(),
			args:    []any{"key", "value"},
			wantErr: true,
			errType: ErrUninitializedContext,
		},
		{
			name:    "empty args",
			ctx:     NewContext(context.Background()),
			args:    []any{},
			wantErr: false,
		},
		{
			name:    "nil context",
			ctx:     nil,
			args:    []any{"key", "value"},
			wantErr: true,
			errType: ErrUninitializedContext,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := AddFields(tt.ctx, tt.args...)
			if (err != nil) != tt.wantErr {
				t.Errorf("AddFields() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.errType != nil && err != tt.errType {
				if _, ok := err.(*ErrInvalidKey); !ok || tt.name != "non-string key" {
					t.Errorf("AddFields() error = %v, want %v", err, tt.errType)
				}
			}
		})
	}
}

func TestAddWarning(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() context.Context
		message string
		fields  []any
		wantErr bool
	}{
		{
			name: "add warning with no fields",
			setup: func() context.Context {
				return NewContext(context.Background())
			},
			message: "something went wrong",
			fields:  nil,
			wantErr: false,
		},
		{
			name: "add warning with fields",
			setup: func() context.Context {
				return NewContext(context.Background())
			},
			message: "slow query",
			fields:  []any{"duration_ms", 2500, "threshold_ms", 1000},
			wantErr: false,
		},
		{
			name: "uninitialized context",
			setup: func() context.Context {
				return context.Background()
			},
			message: "warning",
			fields:  nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setup()
			err := AddWarning(ctx, tt.message, tt.fields...)

			if (err != nil) != tt.wantErr {
				t.Errorf("AddWarning() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				container := getContainer(ctx)
				if len(container.warnings) != 1 {
					t.Errorf("Expected 1 warning, got %d", len(container.warnings))
				}
				if container.warnings[0].Message != tt.message {
					t.Errorf("Expected message %q, got %q", tt.message, container.warnings[0].Message)
				}
			}
		})
	}
}

func TestAddError(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() context.Context
		message string
		fields  []any
		wantErr bool
	}{
		{
			name: "add error with no fields",
			setup: func() context.Context {
				return NewContext(context.Background())
			},
			message: "database error",
			fields:  nil,
			wantErr: false,
		},
		{
			name: "add error with fields",
			setup: func() context.Context {
				return NewContext(context.Background())
			},
			message: "connection timeout",
			fields:  []any{"host", "db.example.com", "port", 5432},
			wantErr: false,
		},
		{
			name: "uninitialized context",
			setup: func() context.Context {
				return context.Background()
			},
			message: "error",
			fields:  nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setup()
			err := AddError(ctx, tt.message, tt.fields...)

			if (err != nil) != tt.wantErr {
				t.Errorf("AddError() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				container := getContainer(ctx)
				if len(container.errors) != 1 {
					t.Errorf("Expected 1 error, got %d", len(container.errors))
				}
				if container.errors[0].Message != tt.message {
					t.Errorf("Expected message %q, got %q", tt.message, container.errors[0].Message)
				}
			}
		})
	}
}

func TestHasWarnings(t *testing.T) {
	tests := []struct {
		name  string
		setup func() context.Context
		want  bool
	}{
		{
			name: "context with warnings",
			setup: func() context.Context {
				ctx := NewContext(context.Background())
				AddWarning(ctx, "warning 1")
				return ctx
			},
			want: true,
		},
		{
			name: "context without warnings",
			setup: func() context.Context {
				return NewContext(context.Background())
			},
			want: false,
		},
		{
			name: "uninitialized context",
			setup: func() context.Context {
				return context.Background()
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setup()
			if got := HasWarnings(ctx); got != tt.want {
				t.Errorf("HasWarnings() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasErrors(t *testing.T) {
	tests := []struct {
		name  string
		setup func() context.Context
		want  bool
	}{
		{
			name: "context with errors",
			setup: func() context.Context {
				ctx := NewContext(context.Background())
				AddError(ctx, "error 1")
				return ctx
			},
			want: true,
		},
		{
			name: "context without errors",
			setup: func() context.Context {
				return NewContext(context.Background())
			},
			want: false,
		},
		{
			name: "uninitialized context",
			setup: func() context.Context {
				return context.Background()
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setup()
			if got := HasErrors(ctx); got != tt.want {
				t.Errorf("HasErrors() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLogger(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := New(slog.New(handler))

	ctx := NewContext(context.Background())
	AddFields(ctx, "user_id", 123, "action", "login")

	logger.Info(ctx, "test message", "extra", "field")

	// Parse JSON output
	var result map[string]any
	if err := json.NewDecoder(&buf).Decode(&result); err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	// Verify fields
	if result["msg"] != "test message" {
		t.Errorf("Expected msg='test message', got %v", result["msg"])
	}
	if result["user_id"].(float64) != 123 {
		t.Errorf("Expected user_id=123, got %v", result["user_id"])
	}
	if result["action"] != "login" {
		t.Errorf("Expected action='login', got %v", result["action"])
	}
	if result["extra"] != "field" {
		t.Errorf("Expected extra='field', got %v", result["extra"])
	}
}

func TestLoggerWithWarnings(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := New(slog.New(handler))

	ctx := NewContext(context.Background())
	AddFields(ctx, "request_id", "abc123")
	AddWarning(ctx, "slow query", "duration_ms", 2500)
	AddWarning(ctx, "cache miss", "key", "user:123")

	logger.Info(ctx, "request completed")

	// Parse JSON output
	var result map[string]any
	if err := json.NewDecoder(&buf).Decode(&result); err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	// Verify warnings
	if result["warning_count"].(float64) != 2 {
		t.Errorf("Expected warning_count=2, got %v", result["warning_count"])
	}

	warnings, ok := result["warnings"].([]any)
	if !ok {
		t.Fatal("Expected warnings to be an array")
	}

	if len(warnings) != 2 {
		t.Errorf("Expected 2 warnings, got %d", len(warnings))
	}

	// Check first warning
	warning1 := warnings[0].(map[string]any)
	if warning1["message"] != "slow query" {
		t.Errorf("Expected first warning message='slow query', got %v", warning1["message"])
	}
}

func TestLoggerWithErrors(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := New(slog.New(handler))

	ctx := NewContext(context.Background())
	AddFields(ctx, "request_id", "xyz789")
	AddError(ctx, "database timeout", "host", "db.example.com")
	AddError(ctx, "retry failed", "attempts", 3)

	logger.Error(ctx, "request failed")

	// Parse JSON output
	var result map[string]any
	if err := json.NewDecoder(&buf).Decode(&result); err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	// Verify errors
	if result["error_count"].(float64) != 2 {
		t.Errorf("Expected error_count=2, got %v", result["error_count"])
	}

	errors, ok := result["errors"].([]any)
	if !ok {
		t.Fatal("Expected errors to be an array")
	}

	if len(errors) != 2 {
		t.Errorf("Expected 2 errors, got %d", len(errors))
	}
}

func TestLoggerWithWarningsAndErrors(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := New(slog.New(handler))

	ctx := NewContext(context.Background())
	AddWarning(ctx, "warning 1")
	AddError(ctx, "error 1")
	AddWarning(ctx, "warning 2")

	logger.Error(ctx, "mixed issues")

	var result map[string]any
	if err := json.NewDecoder(&buf).Decode(&result); err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

	if result["warning_count"].(float64) != 2 {
		t.Errorf("Expected warning_count=2, got %v", result["warning_count"])
	}

	if result["error_count"].(float64) != 1 {
		t.Errorf("Expected error_count=1, got %v", result["error_count"])
	}
}

func TestConcurrentAddFields(t *testing.T) {
	ctx := NewContext(context.Background())

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			AddFields(ctx, "goroutine", id)
		}(i)
	}

	wg.Wait()

	container := getContainer(ctx)
	if container == nil {
		t.Fatal("Container should exist")
	}

	container.mu.Lock()
	defer container.mu.Unlock()

	// Should have one goroutine field (last write wins)
	if _, exists := container.fields["goroutine"]; !exists {
		t.Error("Expected goroutine field to exist")
	}
}

func TestConcurrentWarnings(t *testing.T) {
	ctx := NewContext(context.Background())

	var wg sync.WaitGroup
	numWarnings := 50
	for i := 0; i < numWarnings; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			AddWarning(ctx, "concurrent warning", "id", id)
		}(i)
	}

	wg.Wait()

	container := getContainer(ctx)
	container.mu.Lock()
	defer container.mu.Unlock()

	if len(container.warnings) != numWarnings {
		t.Errorf("Expected %d warnings, got %d", numWarnings, len(container.warnings))
	}
}

func TestConcurrentErrors(t *testing.T) {
	ctx := NewContext(context.Background())

	var wg sync.WaitGroup
	numErrors := 50
	for i := 0; i < numErrors; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			AddError(ctx, "concurrent error", "id", id)
		}(i)
	}

	wg.Wait()

	container := getContainer(ctx)
	container.mu.Lock()
	defer container.mu.Unlock()

	if len(container.errors) != numErrors {
		t.Errorf("Expected %d errors, got %d", numErrors, len(container.errors))
	}
}

func TestCollectFields(t *testing.T) {
	tests := []struct {
		name         string
		setup        func() context.Context
		wantFields   int // total number of elements (keys + values)
		wantWarnings bool
		wantErrors   bool
	}{
		{
			name: "with fields only",
			setup: func() context.Context {
				ctx := NewContext(context.Background())
				AddFields(ctx, "k1", "v1", "k2", "v2")
				return ctx
			},
			wantFields: 4, // 2 key-value pairs = 4 elements
		},
		{
			name: "with fields and warnings",
			setup: func() context.Context {
				ctx := NewContext(context.Background())
				AddFields(ctx, "k1", "v1")
				AddWarning(ctx, "warning")
				return ctx
			},
			wantFields:   6, // k1, v1, warnings, [...], warning_count, 1
			wantWarnings: true,
		},
		{
			name: "with fields and errors",
			setup: func() context.Context {
				ctx := NewContext(context.Background())
				AddFields(ctx, "k1", "v1")
				AddError(ctx, "error")
				return ctx
			},
			wantFields: 6, // k1, v1, errors, [...], error_count, 1
			wantErrors: true,
		},
		{
			name: "empty context",
			setup: func() context.Context {
				return NewContext(context.Background())
			},
			wantFields: 0,
		},
		{
			name: "uninitialized context",
			setup: func() context.Context {
				return context.Background()
			},
			wantFields: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setup()
			fields := collectFields(ctx)

			if len(fields) != tt.wantFields {
				t.Errorf("collectFields() returned %d elements, want %d", len(fields), tt.wantFields)
			}

			// Check for presence of warnings/errors in collected fields
			hasWarnings := false
			hasErrors := false
			for i := 0; i < len(fields); i += 2 {
				if i+1 >= len(fields) {
					break
				}
				key, ok := fields[i].(string)
				if !ok {
					continue
				}
				if key == "warnings" {
					hasWarnings = true
				}
				if key == "errors" {
					hasErrors = true
				}
			}

			if hasWarnings != tt.wantWarnings {
				t.Errorf("Expected warnings=%v, got %v", tt.wantWarnings, hasWarnings)
			}
			if hasErrors != tt.wantErrors {
				t.Errorf("Expected errors=%v, got %v", tt.wantErrors, hasErrors)
			}
		})
	}
}

func TestSetDefaultLogger(t *testing.T) {
	t.Run("sets logger successfully", func(t *testing.T) {
		var buf bytes.Buffer
		handler := slog.NewJSONHandler(&buf, nil)
		customLogger := slog.New(handler)

		SetDefaultLogger(customLogger)

		// Verify by logging something
		ctx := NewContext(context.Background())
		Info(ctx, "test")

		if !strings.Contains(buf.String(), "test") {
			t.Error("Custom logger was not used")
		}
	})

	t.Run("panics on nil logger", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("SetDefaultLogger should panic on nil logger")
			}
		}()
		SetDefaultLogger(nil)
	})
}

func TestPackageLevelFunctions(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	SetDefaultLogger(slog.New(handler))

	ctx := NewContext(context.Background())
	AddFields(ctx, "test", "value")

	// Test all package-level functions
	t.Run("Info", func(t *testing.T) {
		buf.Reset()
		Info(ctx, "info message")
		if !strings.Contains(buf.String(), "info message") {
			t.Error("Info() did not log correctly")
		}
	})

	t.Run("Error", func(t *testing.T) {
		buf.Reset()
		Error(ctx, "error message")
		if !strings.Contains(buf.String(), "error message") {
			t.Error("Error() did not log correctly")
		}
	})

	t.Run("Warn", func(t *testing.T) {
		buf.Reset()
		Warn(ctx, "warn message")
		if !strings.Contains(buf.String(), "warn message") {
			t.Error("Warn() did not log correctly")
		}
	})

	t.Run("Debug", func(t *testing.T) {
		buf.Reset()
		Debug(ctx, "debug message")
		if !strings.Contains(buf.String(), "debug message") {
			t.Error("Debug() did not log correctly")
		}
	})
}

// Benchmarks

func BenchmarkAddFields(b *testing.B) {
	ctx := NewContext(context.Background())
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		AddFields(ctx, "key", i)
	}
}

func BenchmarkAddWarning(b *testing.B) {
	ctx := NewContext(context.Background())
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		AddWarning(ctx, "warning message", "id", i)
	}
}

func BenchmarkAddError(b *testing.B) {
	ctx := NewContext(context.Background())
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		AddError(ctx, "error message", "id", i)
	}
}

func BenchmarkLogWithFields(b *testing.B) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, nil)
	logger := New(slog.New(handler))

	ctx := NewContext(context.Background())
	AddFields(ctx, "k1", "v1", "k2", "v2", "k3", "v3")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		logger.Info(ctx, "benchmark message")
	}
}

func BenchmarkLogWithWarningsAndErrors(b *testing.B) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, nil)
	logger := New(slog.New(handler))

	ctx := NewContext(context.Background())
	AddFields(ctx, "k1", "v1", "k2", "v2")
	AddWarning(ctx, "warning 1", "w1", "v1")
	AddWarning(ctx, "warning 2", "w2", "v2")
	AddError(ctx, "error 1", "e1", "v1")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		logger.Error(ctx, "benchmark message")
	}
}

func BenchmarkConcurrentAddFields(b *testing.B) {
	ctx := NewContext(context.Background())

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			AddFields(ctx, "key", i)
			i++
		}
	})
}

func BenchmarkConcurrentWarnings(b *testing.B) {
	ctx := NewContext(context.Background())

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			AddWarning(ctx, "warning", "id", i)
			i++
		}
	})
}
