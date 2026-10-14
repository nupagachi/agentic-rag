package httpx

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"regexp"
)

// HeaderRequestID is the header used to accept and echo the request id.
const HeaderRequestID = "X-Request-ID"

// validRequestID bounds what we accept from a client (SR-11): no control
// characters, no whitespace, no markup, at most 64 characters. Anything else
// is replaced, never sanitised, so a crafted id cannot reach logs or headers.
var validRequestID = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

type ctxKey int

const (
	requestIDKey ctxKey = iota
	loggerKey
)

// RequestID accepts a well-formed X-Request-ID from the client or generates a
// UUID, stores it in the context, echoes it in the response header and puts a
// logger carrying request_id in the context (see LoggerFromContext).
func RequestID(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get(HeaderRequestID)
			if !validRequestID.MatchString(id) {
				id = newUUID()
			}
			w.Header().Set(HeaderRequestID, id)

			ctx := context.WithValue(r.Context(), requestIDKey, id)
			ctx = context.WithValue(ctx, loggerKey, log.With(slog.String("request_id", id)))
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequestIDFromContext returns the request id set by RequestID, or "".
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

// LoggerFromContext returns the request-scoped logger set by RequestID, or
// slog.Default() when the context did not pass through that middleware.
func LoggerFromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(loggerKey).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}

// newUUID returns a random (version 4) UUID in canonical form.
func newUUID() string {
	var b [16]byte
	// Since Go 1.24 crypto/rand.Read does not return an error; it aborts the
	// program if the system source fails, which is the right outcome here.
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	var out [36]byte
	hex.Encode(out[0:8], b[0:4])
	out[8] = '-'
	hex.Encode(out[9:13], b[4:6])
	out[13] = '-'
	hex.Encode(out[14:18], b[6:8])
	out[18] = '-'
	hex.Encode(out[19:23], b[8:10])
	out[23] = '-'
	hex.Encode(out[24:36], b[10:16])
	return string(out[:])
}
