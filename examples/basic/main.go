package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/mucansever/widelogger"
)

func helloHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID := r.URL.Query().Get("user_id")
	if userID != "" {
		if err := widelogger.AddFields(ctx, "user_id", userID); err != nil {
			widelogger.Error(ctx, "failed to add user_id field", "error", err)
		}
	}

	widelogger.AddFields(ctx, "handler", "helloHandler")

	// Simulate some work
	time.Sleep(200 * time.Millisecond)

	// Log specific events during request processing
	if userID == "" {
		widelogger.AddWarning(ctx, "request missing user_id")
		http.Error(w, "user_id parameter required", http.StatusBadRequest)
		return
	}

	fmt.Fprintf(w, "Hello, %s!", userID)
	// Note: The final request log happens automatically in middleware
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
