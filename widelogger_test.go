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
	// Capture internal logs
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, nil)
	SetDefaultLogger(slog.New(handler))

	tests := []struct {
		name          string
		ctx           context.Context
		args          []any
		wantFields    map[string]any
		wantLogSubstr string
	}{
		{
			name:       "valid key-value pairs",
			ctx:        NewContext(context.Background()),
			args:       []any{"key1", "value1", "key2", 123},
			wantFields: map[string]any{"key1": "value1", "key2": 123},
		},
		{
			name:          "odd number of arguments",
			ctx:           NewContext(context.Background()),
			args:          []any{"key1", "value1", "key2"},
			wantFields:    map[string]any{},
			wantLogSubstr: "odd number of arguments",
		},
		{
			name:          "non-string key",
			ctx:           NewContext(context.Background()),
			args:          []any{123, "value"},
			wantFields:    map[string]any{},
			wantLogSubstr: "key must be string",
		},
		{
			name:          "uninitialized context",
			ctx:           context.Background(),
			args:          []any{"key", "value"},
			wantFields:    nil, // No container to check
			wantLogSubstr: "context not initialized",
		},
		{
			name:       "empty args",
			ctx:        NewContext(context.Background()),
			args:       []any{},
			wantFields: map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			AddFields(tt.ctx, tt.args...)

			if tt.wantFields != nil {
				container := getContainer(tt.ctx)
				if container == nil {
					t.Fatal("Container expected but not found")
				}
				container.mu.Lock()
				if len(container.fields) != len(tt.wantFields) {
					t.Errorf("Expected %d fields, got %d", len(tt.wantFields), len(container.fields))
				}
				for k, v := range tt.wantFields {
					if container.fields[k] != v {
						t.Errorf("Expected field %s=%v, got %v", k, v, container.fields[k])
					}
				}
				container.mu.Unlock()
			}

			if tt.wantLogSubstr != "" {
				if !strings.Contains(buf.String(), tt.wantLogSubstr) {
					t.Errorf("Expected log containing %q, got %q", tt.wantLogSubstr, buf.String())
				}
			} else if buf.Len() > 0 {
				t.Errorf("Expected no log, got %q", buf.String())
			}
		})
	}
}

func TestAddWarning(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, nil)
	SetDefaultLogger(slog.New(handler))

	tests := []struct {
		name          string
		setup         func() context.Context
		message       string
		fields        []any
		wantWarnings  int
		wantLogSubstr string
	}{
		{
			name: "add warning with no fields",
			setup: func() context.Context {
				return NewContext(context.Background())
			},
			message:      "something went wrong",
			fields:       nil,
			wantWarnings: 1,
		},
		{
			name: "add warning with fields",
			setup: func() context.Context {
				return NewContext(context.Background())
			},
			message:      "slow query",
			fields:       []any{"duration_ms", 2500, "threshold_ms", 1000},
			wantWarnings: 1,
		},
		{
			name: "uninitialized context",
			setup: func() context.Context {
				return context.Background()
			},
			message:       "warning",
			fields:        nil,
			wantWarnings:  0,
			wantLogSubstr: "context not initialized",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			ctx := tt.setup()
			AddWarning(ctx, tt.message, tt.fields...)

			if tt.wantLogSubstr != "" {
				if !strings.Contains(buf.String(), tt.wantLogSubstr) {
					t.Errorf("Expected log containing %q, got %q", tt.wantLogSubstr, buf.String())
				}
			}

			if tt.wantWarnings > 0 {
				container := getContainer(ctx)
				if len(container.warnings) != tt.wantWarnings {
					t.Errorf("Expected %d warning, got %d", tt.wantWarnings, len(container.warnings))
				}
				if container.warnings[0].Message != tt.message {
					t.Errorf("Expected message %q, got %q", tt.message, container.warnings[0].Message)
				}
			}
		})
	}
}

func TestAddError(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, nil)
	SetDefaultLogger(slog.New(handler))

	tests := []struct {
		name          string
		setup         func() context.Context
		message       string
		fields        []any
		wantErrors    int
		wantLogSubstr string
	}{
		{
			name: "add error with no fields",
			setup: func() context.Context {
				return NewContext(context.Background())
			},
			message:    "database error",
			fields:     nil,
			wantErrors: 1,
		},
		{
			name: "add error with fields",
			setup: func() context.Context {
				return NewContext(context.Background())
			},
			message:    "connection timeout",
			fields:     []any{"host", "db.example.com", "port", 5432},
			wantErrors: 1,
		},
		{
			name: "uninitialized context",
			setup: func() context.Context {
				return context.Background()
			},
			message:       "error",
			fields:        nil,
			wantErrors:    0,
			wantLogSubstr: "context not initialized",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			ctx := tt.setup()
			AddError(ctx, tt.message, tt.fields...)

			if tt.wantLogSubstr != "" {
				if !strings.Contains(buf.String(), tt.wantLogSubstr) {
					t.Errorf("Expected log containing %q, got %q", tt.wantLogSubstr, buf.String())
				}
			}

			if tt.wantErrors > 0 {
				container := getContainer(ctx)
				if len(container.errors) != tt.wantErrors {
					t.Errorf("Expected %d error, got %d", tt.wantErrors, len(container.errors))
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

	var result map[string]any
	if err := json.NewDecoder(&buf).Decode(&result); err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

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

	var result map[string]any
	if err := json.NewDecoder(&buf).Decode(&result); err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

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

	var result map[string]any
	if err := json.NewDecoder(&buf).Decode(&result); err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}

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

	if _, exists := container.fields["goroutine"]; !exists {
		t.Error("Expected goroutine field to exist")
	}
}

func TestSetDefaultLogger(t *testing.T) {
	t.Run("sets logger successfully", func(t *testing.T) {
		var buf bytes.Buffer
		handler := slog.NewJSONHandler(&buf, nil)
		customLogger := slog.New(handler)

		SetDefaultLogger(customLogger)

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