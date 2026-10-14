package server_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/nupagachi/agentic-rag/api/internal/platform/health"
	"github.com/nupagachi/agentic-rag/api/internal/platform/server"
)

// devOrigins mirrors the config default; the server package no longer owns one.
var devOrigins = []string{"http://localhost:3000", "http://127.0.0.1:3000"}

func jsonLog() (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	return slog.New(slog.NewJSONHandler(&buf, nil)), &buf
}

func logLines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		m := map[string]any{}
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("not JSON: %q", line)
		}
		out = append(out, m)
	}
	return out
}

// TestMiddlewares_OrderAndWiring checks AC5 end to end through NewRouter:
// RequestID is outermost (a panic response still carries the id), AccessLog
// wraps Recover (it logs the 500), and CORS headers reach real routes.
func TestMiddlewares_OrderAndWiring(t *testing.T) {
	t.Parallel()

	log, buf := jsonLog()
	probes := health.NewChecker(nil, time.Second)
	r := server.NewRouter(server.Options{Middlewares: server.Middlewares(log, devOrigins)}, probes)
	r.Get("/boom", func(http.ResponseWriter, *http.Request) { panic("boom") })

	t.Run("probe with default dev origin", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/readyz", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		req.Header.Set("X-Request-ID", "rid-probe")
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d", rec.Code)
		}
		if got := rec.Header().Get("X-Request-ID"); got != "rid-probe" {
			t.Errorf("X-Request-ID = %q", got)
		}
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
			t.Errorf("Allow-Origin = %q", got)
		}
	})

	t.Run("panic: id on response, 500 in access log", func(t *testing.T) {
		buf.Reset()
		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/boom", nil)
		req.Header.Set("X-Request-ID", "rid-panic")
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError || rec.Header().Get("X-Request-ID") != "rid-panic" {
			t.Fatalf("status = %d, id = %q", rec.Code, rec.Header().Get("X-Request-ID"))
		}
		var access map[string]any
		for _, l := range logLines(t, buf) {
			if l["msg"] == "request" {
				access = l
			}
		}
		if access == nil || access["status"] != float64(500) || access["route"] != "/boom" || access["request_id"] != "rid-panic" {
			t.Errorf("access line = %v", access)
		}
	})

	t.Run("preflight short-circuits with request id", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(t.Context(), http.MethodOptions, "/readyz", nil)
		req.Header.Set("Origin", "http://127.0.0.1:3000")
		req.Header.Set("Access-Control-Request-Method", "GET")
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent || rec.Header().Get("X-Request-ID") == "" {
			t.Errorf("status = %d, id = %q", rec.Code, rec.Header().Get("X-Request-ID"))
		}
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://127.0.0.1:3000" {
			t.Errorf("Allow-Origin = %q", got)
		}
	})
}

func TestMiddlewares_OnlyConfiguredOriginsAllowed(t *testing.T) {
	t.Parallel()

	log, _ := jsonLog()
	r := chi.NewRouter()
	r.Use(server.Middlewares(log, []string{"https://app.example.com"})...)
	r.Get("/x", func(http.ResponseWriter, *http.Request) {})

	for origin, want := range map[string]string{
		"https://app.example.com": "https://app.example.com",
		"http://localhost:3000":   "",
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/x", nil)
		req.Header.Set("Origin", origin)
		r.ServeHTTP(rec, req)
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != want {
			t.Errorf("origin %q: Allow-Origin = %q, want %q", origin, got, want)
		}
	}
}

// An empty list (not what config produces, but possible for a direct caller)
// grants no cross-origin access: there is no hidden fallback (SR-12).
func TestMiddlewares_EmptyOriginsGrantNothing(t *testing.T) {
	t.Parallel()

	log, _ := jsonLog()
	r := chi.NewRouter()
	r.Use(server.Middlewares(log, nil)...)
	r.Get("/x", func(http.ResponseWriter, *http.Request) {})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/x", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	r.ServeHTTP(rec, req)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Allow-Origin = %q, want empty", got)
	}
}

func TestMiddlewares_Length(t *testing.T) {
	t.Parallel()

	log, _ := jsonLog()
	if n := len(server.Middlewares(log, nil)); n != 4 {
		t.Fatalf("len = %d, want 4 (RequestID, AccessLog, Recover, CORS)", n)
	}
}
