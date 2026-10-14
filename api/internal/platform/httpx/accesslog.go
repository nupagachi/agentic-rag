package httpx

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// RouteUnmatched is logged as the route when no chi pattern matched (404/405
// and short-circuited preflights). The raw URL is never logged (SR-10).
const RouteUnmatched = "unmatched"

// AccessLog writes one JSON line per request with exactly these keys:
// request_id, method, route, status, duration_ms, bytes (plus the handler's
// time, level and msg). It never logs headers, query strings, paths or
// bodies (SR-10), so Authorization, Cookie and tokens in URLs cannot leak.
//
// It must be registered inside the chi router (Router.Use) so the route
// pattern is known once the handler returns, and outside Recover so it sees
// the 500 that Recover writes.
func AccessLog(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := &statusWriter{ResponseWriter: w}

			// Log from a defer so a re-raised http.ErrAbortHandler (SR-9) is
			// still recorded before it propagates.
			defer func() {
				rec := recover()
				status := rw.status
				if status == 0 {
					status = http.StatusOK
					if rec != nil {
						// Aborted before any byte was written: the client sees a
						// broken connection; 500 is the closest honest value.
						status = http.StatusInternalServerError
					}
				}
				logRequest(r, log, rw, status, start)
				if rec != nil {
					panic(rec)
				}
			}()

			next.ServeHTTP(rw, r)
		})
	}
}

func logRequest(r *http.Request, log *slog.Logger, rw *statusWriter, status int, start time.Time) {
	route := RouteUnmatched
	if rctx := chi.RouteContext(r.Context()); rctx != nil {
		if p := rctx.RoutePattern(); p != "" {
			route = p
		}
	}
	log.LogAttrs(context.WithoutCancel(r.Context()), slog.LevelInfo, "request",
		slog.String("request_id", RequestIDFromContext(r.Context())),
		slog.String("method", r.Method),
		slog.String("route", route),
		slog.Int("status", status),
		slog.Float64("duration_ms", float64(time.Since(start).Microseconds())/1000),
		slog.Int64("bytes", rw.bytes),
	)
}

// statusWriter records the status code and body size. It forwards Flush and
// exposes the wrapped writer through Unwrap, so http.ResponseController
// (deadlines, flushing for SSE) keeps working.
type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int64
}

func (w *statusWriter) WriteHeader(code int) {
	// Informational responses (1xx) do not end the header phase.
	if w.status == 0 && (code < 100 || code > 199 || code == http.StatusSwitchingProtocols) {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(p)
	w.bytes += int64(n)
	return n, err //nolint:wrapcheck // transparent io.Writer
}

// Flush implements http.Flusher.
func (w *statusWriter) Flush() {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
