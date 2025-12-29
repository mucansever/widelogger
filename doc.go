/*
widelogger allows you to accumulate log fields throughout a request lifecycle and emit them all at once, providing complete context in a single log entry.

# Basic Usage

	ctx := widelogger.NewContext(context.Background())
	widelogger.AddFields(ctx, "user_id", 123)
	widelogger.Info(ctx, "user logged in")

# HTTP Middleware

	mux := http.NewServeMux()
	handler := widelogger.SimpleHTTPMiddleware(mux)
	http.ListenAndServe(":8080", handler)

# Accumulating Warnings and Errors

Instead of logging immediately, accumulate warnings and errors:

	widelogger.AddWarning(ctx, "slow query", "duration_ms", 2500)
	widelogger.AddError(ctx, "database timeout")
	// Single log entry at the end with all context

For more examples, see the examples/ directory or visit:
https://github.com/mucansever/widelogger
*/
package widelogger
