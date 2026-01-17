package widelogger

import (
	"context"
	"log/slog"
	"math/rand/v2"
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

type config struct {
	logger         *Logger
	includeHeaders []string
	excludePaths   map[string]bool
	onPanic        func(context.Context, any)
	samplingRate   float64
}

type Option func(*config)

func WithLogger(l *Logger) Option {
	return func(c *config) {
		c.logger = l
	}
}

func WithIncludeRequestHeaders(headers ...string) Option {
	return func(c *config) {
		c.includeHeaders = append(c.includeHeaders, headers...)
	}
}

func WithExcludePaths(paths ...string) Option {
	return func(c *config) {
		if c.excludePaths == nil {
			c.excludePaths = make(map[string]bool)
		}
		for _, p := range paths {
			c.excludePaths[p] = true
		}
	}
}

func WithPanicHandler(fn func(context.Context, any)) Option {
	return func(c *config) {
		c.onPanic = fn
	}
}

func WithSuccessSampling(rate float64) Option {
	return func(c *config) {
		if rate < 0 {
			rate = 0
		} else if rate > 1 {
			rate = 1
		}
		c.samplingRate = rate
	}
}

func Middleware(next http.Handler, opts ...Option) http.Handler {
	cfg := &config{
		samplingRate: 1.0,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	if cfg.logger == nil {
		cfg.logger = New(nil)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cfg.excludePaths != nil && cfg.excludePaths[r.URL.Path] {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		ctx := NewContext(r.Context())

		wrapped := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		AddFields(ctx,
			"method", r.Method,
			"path", r.URL.Path,
			"remote_addr", r.RemoteAddr,
		)

		if r.URL.RawQuery != "" {
			AddFields(ctx, "query", r.URL.RawQuery)
		}

		if len(cfg.includeHeaders) > 0 {
			headers := make(map[string]string, len(cfg.includeHeaders))
			for _, headerName := range cfg.includeHeaders {
				if val := r.Header.Get(headerName); val != "" {
					headers[headerName] = val
				}
			}
			if len(headers) > 0 {
				AddFields(ctx, "request_headers", headers)
			}
		}

		defer func() {
			if recovered := recover(); recovered != nil {
				AddFields(ctx, "panic", recovered)
				cfg.logger.Error(ctx, "http_request_panic")

				if cfg.onPanic != nil {
					cfg.onPanic(ctx, recovered)
				} else {
					panic(recovered)
				}
			}
		}()

		next.ServeHTTP(wrapped, r.WithContext(ctx))

		duration := time.Since(start)
		AddFields(ctx,
			"status_code", wrapped.statusCode,
			"duration_ms", duration.Milliseconds(),
		)

		if err := ctx.Err(); err != nil {
			AddFields(ctx, "context_error", err.Error())
		}

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

		shouldLog := true
		// only sample if not error/warning
		if logLevel == slog.LevelInfo && cfg.samplingRate < 1.0 {
			if rand.Float64() > cfg.samplingRate {
				shouldLog = false
			}
		}

		if shouldLog {
			cfg.logger.Log(ctx, logLevel, logMessage)
		}
	})
}
