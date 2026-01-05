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

## Outputs
```json
{
    "time":"2025-12-30T11:19:18.423799+01:00",
    "level":"INFO",
    "msg":"http_request_completed",
    "remote_addr":"[::1]:52842",
    "query":"user_id=mutlu",
    "request_headers":{
        "User-Agent":"curl/8.7.1"
    },
    "user_id":"mutlu",
    "duration_ms":201,
    "method":"GET",
    "path":"/hello",
    "handler":"helloHandler",
    "status_code":200
}
{
    "time":"2025-12-30T11:19:22.812664+01:00",
    "level":"WARN",
    "msg":"http_request_completed_with_warnings",
    "request_headers":{
        "User-Agent":"curl/8.7.1"
    },
    "handler":"helloHandler",
    "status_code":400,
    "duration_ms":201,
    "method":"GET",
    "path":"/hello",
    "remote_addr":"[::1]:52843",
    "query":"user_id=",
    "warnings":[
        {"message":"request missing user_id"}
    ],
    "warning_count":1
}
```

## HTTP Middleware
```go
mux := http.NewServeMux()
// Use default middleware
handler := widelogger.Middleware(mux)

// Or with options
handler = widelogger.Middleware(mux,
    widelogger.WithIncludeRequestHeaders("User-Agent"),
    widelogger.WithExcludePaths("/health"),
)

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
