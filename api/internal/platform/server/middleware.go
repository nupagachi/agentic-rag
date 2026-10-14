package server

import (
	"log/slog"

	"github.com/nupagachi/agentic-rag/api/internal/platform/httpx"
)

// Middlewares returns the base middleware chain for Options.Middlewares, in
// the required order: RequestID, AccessLog, Recover, CORS (outermost first).
//
// RequestID is first so every later layer, including panics and short-circuited
// preflights, has a request id. AccessLog wraps Recover so it records the 500
// that Recover writes. corsOrigins comes from config, which owns the default;
// an empty list grants no cross-origin access.
func Middlewares(log *slog.Logger, corsOrigins []string) []Middleware {
	return []Middleware{
		httpx.RequestID(log),
		httpx.AccessLog(log),
		httpx.Recover(log),
		httpx.CORS(corsOrigins),
	}
}
