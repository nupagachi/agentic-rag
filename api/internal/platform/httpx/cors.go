package httpx

import (
	"net/http"
	"slices"
	"strings"
)

const (
	corsAllowMethods = "GET, HEAD, OPTIONS"
	corsAllowHeaders = "Content-Type, Authorization, X-Request-ID"
	corsMaxAge       = "600"
)

// CORS allows cross-origin reads from an exact allowlist of origins (SR-12).
//
//   - Only origins in the list are echoed; "*" and "null" are never accepted
//     (they are dropped if present in the list).
//   - Credentials are never allowed: no Access-Control-Allow-Credentials.
//   - Methods are limited to GET, HEAD and OPTIONS. State-changing methods get
//     no CORS grant until a later task needs them and the threat model is
//     updated.
//   - Vary: Origin is always set so caches do not mix per-origin answers.
//
// An empty allowlist grants nothing. The default allowlist lives in config
// (config.DefaultCORSOrigins), not here.
//
// A preflight (OPTIONS with Origin and Access-Control-Request-Method) is
// answered here with 204 and never reaches the router.
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	allowed := make([]string, 0, len(allowedOrigins))
	for _, o := range allowedOrigins {
		o = strings.TrimSpace(o)
		if o == "" || o == "*" || strings.EqualFold(o, "null") {
			continue
		}
		allowed = append(allowed, o)
	}
	methods := strings.Split(corsAllowMethods, ", ")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Add("Vary", "Origin")

			origin := r.Header.Get("Origin")
			ok := origin != "" && slices.Contains(allowed, origin)

			reqMethod := r.Header.Get("Access-Control-Request-Method")
			if r.Method == http.MethodOptions && origin != "" && reqMethod != "" {
				h.Add("Vary", "Access-Control-Request-Method")
				h.Add("Vary", "Access-Control-Request-Headers")
				if ok && slices.Contains(methods, reqMethod) {
					h.Set("Access-Control-Allow-Origin", origin)
					h.Set("Access-Control-Allow-Methods", corsAllowMethods)
					h.Set("Access-Control-Allow-Headers", corsAllowHeaders)
					h.Set("Access-Control-Max-Age", corsMaxAge)
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}

			if ok {
				h.Set("Access-Control-Allow-Origin", origin)
				h.Set("Access-Control-Expose-Headers", HeaderRequestID)
			}
			next.ServeHTTP(w, r)
		})
	}
}
