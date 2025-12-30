# widelogger

Context-based structured logging for Go with field accumulation. Collects log fields throughout a request lifecycle and emits them all at once. 

Inspired by: https://loggingsucks.com

## Installation

```bash
go get github.com/mucansever/widelogger
```

## Basic Usage
```go
func helloHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID := r.URL.Query().Get("user_id")
	if userID != "" {
		if err := widelogger.AddFields(ctx, "user_id", userID); err != nil {
			widelogger.Error(ctx, "failed to add user_id field", "error", err)
		}
	}

	widelogger.AddFields(ctx, "handler", "helloHandler")

	if userID == "" {
		widelogger.AddWarning(ctx, "request missing user_id")
		http.Error(w, "user_id parameter required", http.StatusBadRequest)
		return
	}

	fmt.Fprintf(w, "Hello, %s!", userID)
	// The final request log happens automatically in middleware
}

func main() {
	// Configure the logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	widelogger.SetDefaultLogger(logger)

	// Setup routes
	mux := http.NewServeMux()
	mux.HandleFunc("/hello", helloHandler)

	// Add health check endpoint (excluded from logging)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "OK")
	})

	// Configure middleware with options
	middleware := widelogger.HTTPMiddleware(&widelogger.HTTPMiddlewareConfig{
		Logger: widelogger.New(logger),
		IncludeRequestHeaders: []string{
			"User-Agent",
			"X-Request-ID",
		},
		ExcludePaths: map[string]bool{
			"/health":  true,
			"/metrics": true,
		},
		OnPanic: func(ctx context.Context, recovered any) {
			widelogger.Error(ctx, "panic recovered in handler",
				"panic", recovered,
				"stack", fmt.Sprintf("%+v", recovered),
			)
		},
	})

	wrappedMux := middleware(mux)

	port := ":8080"
	fmt.Printf("Server starting on %s\n", port)

	if err := http.ListenAndServe(port, wrappedMux); err != nil {
		fmt.Printf("Error starting server: %v\n", err)
		os.Exit(1)
	}
}
```

## Outputs
Output is specifically designed to be queryable. You can see that inference of error/warning analytics is very easy from the logs:
```json
{
    "time": "2025-12-30T11:19:18.423799+01:00",
    "level": "INFO",
    "msg": "http_request_completed",
    "remote_addr": "[::1]:52842",
    "query": "user_id=mutlu",
    "request_headers": {
        "User-Agent": "curl/8.7.1"
    },
    "user_id": "mutlu",
    "duration_ms": 201,
    "method": "GET",
    "path": "/hello",
    "handler": "helloHandler",
    "status_code": 200
}
{
    "time": "2025-12-30T11:19:22.812664+01:00",
    "level": "WARN",
    "msg": "http_request_completed_with_warnings",
    "request_headers": {
        "User-Agent": "curl/8.7.1"
    },
    "handler": "helloHandler",
    "status_code": 400,
    "duration_ms": 201,
    "method": "GET",
    "path": "/hello",
    "remote_addr": "[::1]:52843",
    "query": "user_id=",
    "warnings": [
        {"message": "request missing user_id"}
    ],
    "warning_count": 1
}
```

## Improvements

Please create an issue for any improvement that you might think of. 

## License

MIT
