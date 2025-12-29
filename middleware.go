package widelogger

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.written = true
	}
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}

type HTTPMiddlewareConfig struct {
	Logger *Logger // nil for default

	IncludeRequestHeaders []string

	ExcludePaths map[string]bool

	// OnPanic is called when a panic is recovered. If nil, panics are re-raised.
	OnPanic func(ctx context.Context, recovered any)
}

func HTTPMiddleware(config *HTTPMiddlewareConfig) func(http.Handler) http.Handler {
	if config == nil {
		config = &HTTPMiddlewareConfig{}
	}

	logger := config.Logger
	if logger == nil {
		logger = New(nil)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip excluded paths
			if config.ExcludePaths != nil && config.ExcludePaths[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()
			ctx := NewContext(r.Context())

			// Wrap response writer to capture status
			wrapped := &responseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			// Add basic request fields
			AddFields(ctx,
				"method", r.Method,
				"path", r.URL.Path,
				"remote_addr", r.RemoteAddr,
			)

			// Add query string if present
			if r.URL.RawQuery != "" {
				AddFields(ctx, "query", r.URL.RawQuery)
			}

			// Add configured headers
			if len(config.IncludeRequestHeaders) > 0 {
				headers := make(map[string]string, len(config.IncludeRequestHeaders))
				for _, headerName := range config.IncludeRequestHeaders {
					if val := r.Header.Get(headerName); val != "" {
						headers[headerName] = val
					}
				}
				if len(headers) > 0 {
					AddFields(ctx, "request_headers", headers)
				}
			}

			// Handle panics
			defer func() {
				if recovered := recover(); recovered != nil {
					AddFields(ctx, "panic", recovered)
					logger.Error(ctx, "http_request_panic")

					if config.OnPanic != nil {
						config.OnPanic(ctx, recovered)
					} else {
						panic(recovered) // Re-raise if no handler
					}
				}
			}()

			// Serve request
			next.ServeHTTP(wrapped, r.WithContext(ctx))

			// Log completion
			duration := time.Since(start)
			AddFields(ctx,
				"status_code", wrapped.statusCode,
				"duration_ms", duration.Milliseconds(),
			)

			// Check for context cancellation
			if err := ctx.Err(); err != nil {
				AddFields(ctx, "context_error", err.Error())
			}

			// Determine log level based on accumulated state
			var logLevel slog.Level
			var logMessage string

			switch {
			case HasErrors(ctx):
				logLevel = slog.LevelError
				logMessage = "http_request_completed_with_errors"
			case HasWarnings(ctx):
				logLevel = slog.LevelWarn
				logMessage = "http_request_completed_with_warnings"
			case wrapped.statusCode >= 500:
				logLevel = slog.LevelError
				logMessage = "http_request_completed"
			case wrapped.statusCode >= 400:
				logLevel = slog.LevelWarn
				logMessage = "http_request_completed"
			default:
				logLevel = slog.LevelInfo
				logMessage = "http_request_completed"
			}

			logger.Log(ctx, logLevel, logMessage)
		})
	}
}

// SimpleHTTPMiddleware is a wrapper for HTTPMiddleware with default config.
func SimpleHTTPMiddleware(next http.Handler) http.Handler {
	return HTTPMiddleware(nil)(next)
}
