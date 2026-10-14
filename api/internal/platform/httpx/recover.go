package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
)

// internalError is the fixed 500 body (SR-9). It carries only the request id
// so support can find the stack in the logs; the shape matches the
// problem+json `code` and `requestId` fields M1 will standardise.
type internalError struct {
	Code      string `json:"code"`
	RequestID string `json:"requestId"`
}

// Recover turns a handler panic into a 500 with a fixed JSON body and an
// error log line (with stack) correlated by request_id. The response never
// contains the panic value, stack or file paths.
//
// If the response has already started (header or body bytes sent) a clean 500
// is impossible and appending one would corrupt the stream, so after logging
// Recover panics with http.ErrAbortHandler to make net/http cut the
// connection. http.ErrAbortHandler raised by the handler itself is re-raised
// untouched (SR-9).
func Recover(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			sw := &startedWriter{ResponseWriter: w}
			defer func() {
				rec := recover()
				if rec == nil {
					return
				}
				if err, ok := rec.(error); ok && errors.Is(err, http.ErrAbortHandler) {
					panic(rec)
				}
				respondInternal(ctx, log, sw, r.Method, rec)
			}()
			next.ServeHTTP(sw, r)
		})
	}
}

// respondInternal logs the recovered value with its stack, then either writes
// the fixed 500 body or, when the response already started, aborts the
// connection.
func respondInternal(ctx context.Context, log *slog.Logger, w *startedWriter, method string, rec any) {
	id := RequestIDFromContext(ctx)
	log.LogAttrs(context.WithoutCancel(ctx), slog.LevelError, "panic recovered",
		slog.String("request_id", id),
		slog.String("method", method),
		slog.Bool("response_started", w.started),
		slog.String("panic", fmt.Sprint(rec)),
		slog.String("stack", string(debug.Stack())),
	)

	if w.started {
		panic(http.ErrAbortHandler)
	}

	// Marshalling two strings cannot fail; fall back to a constant anyway.
	body, err := json.Marshal(internalError{Code: "INTERNAL", RequestID: id})
	if err != nil {
		body = []byte(`{"code":"INTERNAL","requestId":""}`)
	}
	h := w.Header()
	h.Set("Content-Type", "application/json")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Del("Content-Length")
	w.WriteHeader(http.StatusInternalServerError)
	_, _ = w.Write(body)
}

// startedWriter records whether the response has begun. It forwards Flush and
// exposes the wrapped writer through Unwrap so http.ResponseController keeps
// working.
type startedWriter struct {
	http.ResponseWriter
	started bool
}

func (w *startedWriter) WriteHeader(code int) {
	// Informational responses (1xx) do not end the header phase.
	if code < 100 || code > 199 || code == http.StatusSwitchingProtocols {
		w.started = true
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *startedWriter) Write(p []byte) (int, error) {
	w.started = true
	n, err := w.ResponseWriter.Write(p)
	return n, err //nolint:wrapcheck // transparent io.Writer
}

// Flush implements http.Flusher; flushing commits the header.
func (w *startedWriter) Flush() {
	w.started = true
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (w *startedWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
