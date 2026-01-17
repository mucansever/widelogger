package widelogger

import (
	"context"
	cryptorand "crypto/rand"
	"fmt"
	"log/slog"
	mathrand "math/rand/v2"
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

type requestIDContextKey struct{}

type RequestIDConfig struct {
	HeaderName          string
	Generator           func() string
	PropagateToResponse bool
}

func defaultRequestIDConfig() *RequestIDConfig {
	return &RequestIDConfig{
		HeaderName:          "X-Request-ID",
		Generator:           generateUUID,
		PropagateToResponse: true,
	}
}

func generateUUID() string {
	var b [16]byte
	_, _ = cryptorand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func GetRequestID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	id, _ := ctx.Value(requestIDContextKey{}).(string)
	return id
}

type config struct {
	logger          *Logger
	includeHeaders  []string
	excludePaths    map[string]bool
	onPanic         func(context.Context, any)
	samplingRate    float64
	requestIDConfig *RequestIDConfig
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

func WithRequestID(cfg ...*RequestIDConfig) Option {
	return func(c *config) {
		res := defaultRequestIDConfig()
		if len(cfg) > 0 && cfg[0] != nil {
			if cfg[0].HeaderName != "" {
				res.HeaderName = cfg[0].HeaderName
			}
			if cfg[0].Generator != nil {
				res.Generator = cfg[0].Generator
			}
			res.PropagateToResponse = cfg[0].PropagateToResponse
		}
		c.requestIDConfig = res
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

		if cfg.requestIDConfig != nil {
			requestID := r.Header.Get(cfg.requestIDConfig.HeaderName)
			if requestID == "" {
				requestID = cfg.requestIDConfig.Generator()
			}
			ctx = context.WithValue(ctx, requestIDContextKey{}, requestID)
			AddFields(ctx, "request_id", requestID)
			if cfg.requestIDConfig.PropagateToResponse {
				w.Header().Set(cfg.requestIDConfig.HeaderName, requestID)
			}
		}

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
			if mathrand.Float64() > cfg.samplingRate {
				shouldLog = false
			}
		}

		if shouldLog {
			cfg.logger.Log(ctx, logLevel, logMessage)
		}
	})
}
