/*
Package widelogger provides context-based structured logging with field accumulation.
It allows you to collect log fields throughout a request lifecycle and emit them as a single,
comprehensive "wide" log entry at the end of the operation.

# Basic Usage

	ctx := widelogger.NewContext(context.Background())
	widelogger.AddFields(ctx, "user_id", 123)
	widelogger.Info(ctx, "user logged in")

# HTTP Middleware

The middleware manages the context lifecycle and automatically logs the request summary:

	mux := http.NewServeMux()
	handler := widelogger.Middleware(mux,
	    widelogger.WithSuccessSampling(0.1), // Log only 10% of successful requests
	)
	http.ListenAndServe(":8080", handler)

# Accumulating Warnings and Errors

Instead of logging immediately, you can accumulate issues that will be aggregated into the final log:

	widelogger.AddWarning(ctx, "slow query", "duration_ms", 2500)
	widelogger.AddError(ctx, "database timeout")

For more examples, see the examples/ directory or visit:
https://github.com/mucansever/widelogger
*/
package widelogger