package httpx_test

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/nupagachi/agentic-rag/api/internal/platform/httpx"
)

func decodeLines(t *testing.T, raw string) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
		if line == "" {
			continue
		}
		m := map[string]any{}
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("log line is not JSON: %q: %v", line, err)
		}
		out = append(out, m)
	}
	return out
}

func accessRouter(log *slog.Logger) *chi.Mux {
	r := chi.NewRouter()
	r.Use(httpx.RequestID(log), httpx.AccessLog(log))
	r.Get("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello"))
	})
	r.Get("/kbs/{kbID}/documents", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})
	r.Get("/boom", func(http.ResponseWriter, *http.Request) { panic(http.ErrAbortHandler) })
	return r
}

func TestAccessLog_Fields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		target    string
		wantRoute string
		wantCode  float64
		wantBytes float64
	}{
		{"plain route", "/readyz", "/readyz", 200, 5},
		{"pattern not url", "/kbs/9f1c-secret-id/documents", "/kbs/{kbID}/documents", 202, 0},
		{"query dropped", "/readyz?token=SENTINEL-QUERY", "/readyz", 200, 5},
		{"unmatched", "/nope/SENTINEL-PATH", "unmatched", 404, 19},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			log, buf := jsonLogger()
			rec := get(t, accessRouter(log), tt.target, map[string]string{httpx.HeaderRequestID: "rid-1"})

			lines := decodeLines(t, buf.String())
			if len(lines) != 1 {
				t.Fatalf("got %d log lines, want 1: %s", len(lines), buf.String())
			}
			l := lines[0]
			if l["request_id"] != "rid-1" || l["method"] != "GET" || l["route"] != tt.wantRoute {
				t.Errorf("line = %v", l)
			}
			if l["status"] != tt.wantCode || l["bytes"] != tt.wantBytes {
				t.Errorf("status/bytes = %v/%v, want %v/%v", l["status"], l["bytes"], tt.wantCode, tt.wantBytes)
			}
			if d, ok := l["duration_ms"].(float64); !ok || d < 0 {
				t.Errorf("duration_ms = %v", l["duration_ms"])
			}
			if rec.Code != int(tt.wantCode) {
				t.Errorf("response status = %d", rec.Code)
			}
		})
	}
}

// TestAccessLog_Allowlist is SR-10: only allowlisted keys, and no sentinel from
// the query, path, Authorization, Cookie or body ever appears.
func TestAccessLog_Allowlist(t *testing.T) {
	t.Parallel()

	log, buf := jsonLogger()
	r := accessRouter(log)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
		"/nope/SENTINEL-PATH?token=SENTINEL-QUERY", strings.NewReader("SENTINEL-BODY"))
	req.Header.Set("Authorization", "Bearer SENTINEL-AUTH")
	req.Header.Set("Cookie", "sid=SENTINEL-COOKIE")
	req.Header.Set("User-Agent", "SENTINEL-UA")
	r.ServeHTTP(httptest.NewRecorder(), req)

	out := buf.String()
	for _, s := range []string{"SENTINEL-PATH", "SENTINEL-QUERY", "SENTINEL-AUTH", "SENTINEL-COOKIE", "SENTINEL-BODY", "SENTINEL-UA"} {
		if strings.Contains(out, s) {
			t.Errorf("access log leaks %s: %s", s, out)
		}
	}
	allowed := []string{"time", "level", "msg", "request_id", "method", "route", "status", "duration_ms", "bytes"}
	for _, l := range decodeLines(t, buf.String()) {
		for k := range l {
			if !slices.Contains(allowed, k) {
				t.Errorf("unexpected key %q in access log", k)
			}
		}
		if l["route"] != "unmatched" {
			t.Errorf("route = %v, want unmatched", l["route"])
		}
	}
}

// A handler that aborts must still be logged and the abort must propagate so
// net/http can close the connection.
func TestAccessLog_AbortStillLogsAndRepanics(t *testing.T) {
	t.Parallel()

	log, buf := jsonLogger()
	r := accessRouter(log)

	var got any
	func() {
		defer func() { got = recover() }()
		r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/boom", nil))
	}()

	if err, ok := got.(error); !ok || !errors.Is(err, http.ErrAbortHandler) {
		t.Fatalf("recovered %v, want http.ErrAbortHandler", got)
	}
	lines := decodeLines(t, buf.String())
	if len(lines) != 1 || lines[0]["route"] != "/boom" || lines[0]["status"] != float64(500) {
		t.Errorf("lines = %v", lines)
	}
}

func TestAccessLog_FlushAndUnwrap(t *testing.T) {
	t.Parallel()

	log, buf := jsonLogger()
	r := chi.NewRouter()
	r.Use(httpx.AccessLog(log))
	r.Get("/sse", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("data: 1\n\n"))
		if err := http.NewResponseController(w).Flush(); err != nil {
			t.Errorf("Flush through wrapper: %v", err)
		}
	})
	rec := get(t, r, "/sse", nil)

	if !rec.Flushed {
		t.Error("underlying recorder was not flushed")
	}
	if l := decodeLines(t, buf.String()); len(l) != 1 || l[0]["bytes"] != float64(9) {
		t.Errorf("lines = %v", l)
	}
}
