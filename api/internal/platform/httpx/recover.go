package httpx

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
)

// internalErrorPrefix starts the fixed 500 body (SR-9). The body carries only
// the request id so support can find the stack in the logs; the shape matches
// the problem+json `code` and `requestId` fields M1 will standardise.
const internalErrorPrefix = `{"code":"INTERNAL","requestId":`

// Recover turns a handler panic into a 500 with a fixed JSON body and an
// error log line (with stack) correlated by request_id. The response never
// contains the panic value, stack or file paths.
//
// http.ErrAbortHandler is re-raised untouched: it is the documented way for a
// handler to abort a response, and net/http handles it silently (SR-9).
func Recover(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			defer func() {
				rec := recover()
				if rec == nil {
					return
				}
				if err, ok := rec.(error); ok && errors.Is(err, http.ErrAbortHandler) {
					panic(rec)
				}
				respondInternal(ctx, log, w, r.Method, rec)
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// respondInternal logs the recovered value with its stack and writes the fixed
// 500 body.
func respondInternal(ctx context.Context, log *slog.Logger, w http.ResponseWriter, method string, rec any) {
	id := RequestIDFromContext(ctx)
	log.LogAttrs(context.WithoutCancel(ctx), slog.LevelError, "panic recovered",
		slog.String("request_id", id),
		slog.String("method", method),
		slog.String("panic", fmt.Sprint(rec)),
		slog.String("stack", string(debug.Stack())),
	)

	// If the handler already started the response the status line is gone;
	// the extra WriteHeader is ignored by net/http and the log line stands.
	h := w.Header()
	h.Set("Content-Type", "application/json")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Del("Content-Length")
	w.WriteHeader(http.StatusInternalServerError)
	// id is validated or generated, so it needs no JSON escaping.
	_, _ = fmt.Fprintf(w, "%s%q}", internalErrorPrefix, id)
}
