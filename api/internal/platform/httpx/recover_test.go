package httpx_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/nupagachi/agentic-rag/api/internal/platform/httpx"
)

func recoverRouter(t *testing.T, panicValue any) (*chi.Mux, func() string) {
	t.Helper()
	log, buf := jsonLogger()
	r := chi.NewRouter()
	r.Use(httpx.RequestID(log), httpx.Recover(log))
	r.Get("/boom", func(http.ResponseWriter, *http.Request) { panic(panicValue) })
	r.Get("/ok", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("fine")) })
	return r, buf.String
}

func TestRecover_500ThenServerKeepsServing(t *testing.T) {
	t.Parallel()

	r, logs := recoverRouter(t, "kaboom")
	rec := get(t, r, "/boom", map[string]string{httpx.HeaderRequestID: "rid-7"})

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not JSON: %q: %v", rec.Body.String(), err)
	}
	if len(body) != 2 || body["code"] != "INTERNAL" || body["requestId"] != "rid-7" {
		t.Errorf("body = %v", body)
	}

	// Error log has the stack and the same request id.
	lines := decodeLines(t, logs())
	if len(lines) != 1 {
		t.Fatalf("log lines = %d", len(lines))
	}
	l := lines[0]
	if l["level"] != "ERROR" || l["request_id"] != "rid-7" {
		t.Errorf("log = %v", l)
	}
	if st, _ := l["stack"].(string); !strings.Contains(st, "recover_test.go") {
		t.Errorf("log stack missing: %v", l["stack"])
	}

	// The next request is served normally.
	next := get(t, r, "/ok", nil)
	if next.Code != http.StatusOK || next.Body.String() != "fine" {
		t.Errorf("next request = %d %q", next.Code, next.Body.String())
	}
}

// TestRecover_NoLeak is SR-9: nothing from the panic reaches the client.
func TestRecover_NoLeak(t *testing.T) {
	t.Parallel()

	const secret = "SENTINEL-PANIC-VALUE postgres://user:pw@host/db"
	for name, v := range map[string]any{
		"string": secret,
		"error":  errors.New(secret),
		"struct": struct{ S string }{secret},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r, _ := recoverRouter(t, v)
			rec := get(t, r, "/boom", nil)

			wire := rec.Body.String()
			for k, vs := range rec.Header() {
				wire += k + ": " + strings.Join(vs, ",") + "\n"
			}
			for _, bad := range []string{"SENTINEL", "postgres://", "goroutine", ".go:", "runtime.", "/api/", "panic"} {
				if strings.Contains(wire, bad) {
					t.Errorf("response leaks %q: %s", bad, wire)
				}
			}
		})
	}
}

func TestRecover_ErrAbortHandler_Repanics(t *testing.T) {
	t.Parallel()

	r, logs := recoverRouter(t, http.ErrAbortHandler)
	var got any
	rec := httptest.NewRecorder()
	func() {
		defer func() { got = recover() }()
		r.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/boom", nil))
	}()

	if err, ok := got.(error); !ok || !errors.Is(err, http.ErrAbortHandler) {
		t.Fatalf("recovered %v, want http.ErrAbortHandler", got)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body written for aborted handler: %q", rec.Body.String())
	}
	if strings.Contains(logs(), "panic recovered") {
		t.Error("ErrAbortHandler must not be logged as a recovered panic")
	}
}
