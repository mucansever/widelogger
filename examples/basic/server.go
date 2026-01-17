package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/mucansever/widelogger"
)

func helloHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID := r.URL.Query().Get("user_id")
	if userID != "" {
		widelogger.AddFields(ctx, "user_id", userID)
	}

	widelogger.AddFields(ctx, "handler", "helloHandler")

	if userID == "" {
		widelogger.AddWarning(ctx, "request missing user_id")
		http.Error(w, "user_id parameter required", http.StatusBadRequest)
		return
	}

	fmt.Fprintf(w, "Hello, %s!", userID)
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	widelogger.SetDefaultLogger(logger)

	mux := http.NewServeMux()
	mux.HandleFunc("/hello", helloHandler)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "OK")
	})

	handler := widelogger.Middleware(mux,
		widelogger.WithLogger(widelogger.New(logger)),
		widelogger.WithRequestID(), // enable request ID generation/extraction
		widelogger.WithIncludeRequestHeaders("User-Agent", "X-Request-ID"),
		widelogger.WithExcludePaths("/health", "/metrics"),
		widelogger.WithSuccessSampling(0.5), // log only 50% of successful requests
		widelogger.WithPanicHandler(func(ctx context.Context, recovered any) {
			widelogger.Error(ctx, "panic recovered in handler",
				"panic", recovered,
				"stack", fmt.Sprintf("%+v", recovered),
			)
		}),
	)

	port := ":8080"
	fmt.Printf("Server starting on %s\n", port)

	if err := http.ListenAndServe(port, handler); err != nil {
		fmt.Printf("Error starting server: %v\n", err)
		os.Exit(1)
	}
}
