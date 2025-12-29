# widelogger

Context-based structured logging for Go with field accumulation. Collects log fields throughout a request lifecycle and emits them all at once. 

Inspired by: https://loggingsucks.com

## Installation

```bash
go get github.com/mucansever/widelogger
```

## Basic Usage
```go
ctx := widelogger.NewContext(context.Background())
widelogger.AddFields(ctx, "user_id", 123)
widelogger.Info(ctx, "user logged in")
```

## HTTP Middleware
```go
mux := http.NewServeMux()
handler := widelogger.SimpleHTTPMiddleware(mux)
http.ListenAndServe(":8080", handler)
```

## Accumulating Warnings and Errors

Instead of logging immediately, accumulate warnings and errors:
```go
widelogger.AddWarning(ctx, "slow query", "duration_ms", 2500)
widelogger.AddError(ctx, "database timeout")
// Single log entry at the end with all context
```

For more examples, see the `examples/` directory.

## License

MIT