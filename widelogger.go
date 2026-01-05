package widelogger

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
)

type contextKey struct{}

var (
	fieldsContextKey = contextKey{}
	defaultLogger    = slog.New(slog.NewJSONHandler(os.Stdout, nil))
	defaultLoggerMu  sync.RWMutex
)

// SetDefaultLogger sets the global default logger used by package-level functions.
func SetDefaultLogger(logger *slog.Logger) {
	if logger == nil {
		panic("widelogger: logger cannot be nil")
	}
	defaultLoggerMu.Lock()
	defaultLogger = logger
	defaultLoggerMu.Unlock()
}

func getDefaultLogger() *slog.Logger {
	defaultLoggerMu.RLock()
	defer defaultLoggerMu.RUnlock()
	return defaultLogger
}

// Warning represents a non-fatal issue that occurred during request processing.
type Warning struct {
	Message string         `json:"message"`
	Fields  map[string]any `json:"fields,omitempty"`
}

type fieldContainer struct {
	mu       sync.Mutex
	fields   map[string]any
	warnings []Warning
	errors   []Warning
}

type Logger struct {
	logger *slog.Logger
}

func New(logger *slog.Logger) *Logger {
	if logger == nil {
		logger = getDefaultLogger()
	}
	return &Logger{logger: logger}
}

// NewContext initializes a new context with field accumulation support.
// This must be called before using AddFields or any logging functions.
func NewContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, fieldsContextKey, &fieldContainer{
		fields:   make(map[string]any),
		warnings: make([]Warning, 0),
		errors:   make([]Warning, 0),
	})
}

// AddFields adds key-value pairs to the context for later logging.
// Keys must be strings.
func AddFields(ctx context.Context, keysAndValues ...any) {
	if len(keysAndValues) == 0 {
		return
	}

	container := getContainer(ctx)
	if container == nil {
		getDefaultLogger().WarnContext(ctx, "widelogger: context not initialized", "func", "AddFields")
		return
	}

	if len(keysAndValues)%2 != 0 {
		getDefaultLogger().WarnContext(ctx, "widelogger: odd number of arguments", "func", "AddFields", "args_len", len(keysAndValues))
		return
	}

	container.mu.Lock()
	defer container.mu.Unlock()

	for i := 0; i < len(keysAndValues); i += 2 {
		key, ok := keysAndValues[i].(string)
		if !ok {
			getDefaultLogger().WarnContext(ctx, "widelogger: key must be string", "key_type", fmt.Sprintf("%T", keysAndValues[i]))
			continue
		}
		container.fields[key] = keysAndValues[i+1]
	}
}

// AddWarning accumulates a warning without immediately logging it.
// The warning will be included in the final log entry.
func AddWarning(ctx context.Context, message string, keysAndValues ...any) {
	container := getContainer(ctx)
	if container == nil {
		getDefaultLogger().WarnContext(ctx, "widelogger: context not initialized", "func", "AddWarning")
		return
	}

	warning := Warning{Message: message}
	if len(keysAndValues) > 0 {
		warning.Fields = make(map[string]any)
		for i := 0; i < len(keysAndValues)-1; i += 2 {
			if key, ok := keysAndValues[i].(string); ok {
				warning.Fields[key] = keysAndValues[i+1]
			}
		}
	}

	container.mu.Lock()
	container.warnings = append(container.warnings, warning)
	container.mu.Unlock()
}

// AddError accumulates an error without immediately logging it.
func AddError(ctx context.Context, message string, keysAndValues ...any) {
	container := getContainer(ctx)
	if container == nil {
		getDefaultLogger().WarnContext(ctx, "widelogger: context not initialized", "func", "AddError")
		return
	}

	errEntry := Warning{Message: message}
	if len(keysAndValues) > 0 {
		errEntry.Fields = make(map[string]any)
		for i := 0; i < len(keysAndValues)-1; i += 2 {
			if key, ok := keysAndValues[i].(string); ok {
				errEntry.Fields[key] = keysAndValues[i+1]
			}
		}
	}

	container.mu.Lock()
	container.errors = append(container.errors, errEntry)
	container.mu.Unlock()
}

func HasWarnings(ctx context.Context) bool {
	container := getContainer(ctx)
	if container == nil {
		return false
	}
	container.mu.Lock()
	defer container.mu.Unlock()
	return len(container.warnings) > 0
}

func HasErrors(ctx context.Context) bool {
	container := getContainer(ctx)
	if container == nil {
		return false
	}
	container.mu.Lock()
	defer container.mu.Unlock()
	return len(container.errors) > 0
}

func getContainer(ctx context.Context) *fieldContainer {
	if ctx == nil {
		return nil
	}
	container, _ := ctx.Value(fieldsContextKey).(*fieldContainer)
	return container
}

// collectFields gathers all accumulated fields, warnings, and errors from the context.
func collectFields(ctx context.Context) []any {
	container := getContainer(ctx)
	if container == nil {
		return nil
	}

	container.mu.Lock()
	defer container.mu.Unlock()

	attrs := make([]any, 0, len(container.fields)*2+4)

	for k, v := range container.fields {
		attrs = append(attrs, k, v)
	}

	if len(container.warnings) > 0 {
		attrs = append(attrs, "warnings", container.warnings)
		attrs = append(attrs, "warning_count", len(container.warnings))
	}

	if len(container.errors) > 0 {
		attrs = append(attrs, "errors", container.errors)
		attrs = append(attrs, "error_count", len(container.errors))
	}

	return attrs
}

// Log emits a log with accumulated context fields plus additional fields.
func (l *Logger) Log(ctx context.Context, level slog.Level, msg string, additionalFields ...any) {
	contextFields := collectFields(ctx)

	var allFields []any
	if len(contextFields) > 0 {
		allFields = make([]any, 0, len(contextFields)+len(additionalFields))
		allFields = append(allFields, contextFields...)
		allFields = append(allFields, additionalFields...)
	} else {
		allFields = additionalFields
	}

	l.logger.Log(ctx, level, msg, allFields...)
}

func (l *Logger) Info(ctx context.Context, msg string, additionalFields ...any) {
	l.Log(ctx, slog.LevelInfo, msg, additionalFields...)
}

func (l *Logger) Error(ctx context.Context, msg string, additionalFields ...any) {
	l.Log(ctx, slog.LevelError, msg, additionalFields...)
}

func (l *Logger) Warn(ctx context.Context, msg string, additionalFields ...any) {
	l.Log(ctx, slog.LevelWarn, msg, additionalFields...)
}

func (l *Logger) Debug(ctx context.Context, msg string, additionalFields ...any) {
	l.Log(ctx, slog.LevelDebug, msg, additionalFields...)
}

func Info(ctx context.Context, msg string, additionalFields ...any) {
	New(nil).Info(ctx, msg, additionalFields...)
}

func Error(ctx context.Context, msg string, additionalFields ...any) {
	New(nil).Error(ctx, msg, additionalFields...)
}

func Warn(ctx context.Context, msg string, additionalFields ...any) {
	New(nil).Warn(ctx, msg, additionalFields...)
}

func Debug(ctx context.Context, msg string, additionalFields ...any) {
	New(nil).Debug(ctx, msg, additionalFields...)
}
