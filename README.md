# widelogger

Context-based structured logging for Go with field accumulation. Collects log fields throughout a request lifecycle and emits them all at once. 

Inspired by: https://loggingsucks.com

## Installation
```bash
go get github.com/mucansever/widelogger
```

## Basic Usage
Accumulate fields in `context.Context` and log them together.

```go
ctx := widelogger.NewContext(context.Background())
widelogger.AddFields(ctx, "user_id", 123)
widelogger.AddWarning(ctx, "slow query", "ms", 500)
widelogger.Info(ctx, "request completed")
```

## HTTP Middleware
You can transform your HTTP request lifecycles into widelogs easily with the middleware.

```go
mux := http.NewServeMux()

handler := widelogger.Middleware(mux,
    widelogger.WithIncludeRequestHeaders("User-Agent"),
    widelogger.WithExcludePaths("/health"),
    // log only 10% of successful requests to save space.
    // requests with warnings, errors, or status >= 400 are always logged.
    widelogger.WithSuccessSampling(0.1), 
)

http.ListenAndServe(":8080", handler)
```

## Why widelogger?
Instead of multiple scattered log lines, you get one "wide" log entry containing everything that happened during that request.

```json
{
  "time": "2026-01-17T12:00:00Z",
  "level": "INFO",
  "msg": "http_request_completed",
  "method": "GET",
  "path": "/api/user",
  "user_id": 123,
  "duration_ms": 45,
  "status_code": 200,
  "warnings": [{"message": "slow query", "fields": {"ms": 500}}]
}
```

You can play with the sample server provided in `examples/basic/server.go` by running it with `go run examples/basic/server.go`.

