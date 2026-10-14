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

// A panic after the response started cannot become a clean 500: Recover must
// not append a body and must abort the connection, while the access log still
// records the request.
func TestRecover_ResponseStarted_AbortsWithoutBody(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantBody   string
		wantStatus int
	}{
		{"partial body", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("partial"))
			panic("mid-body")
		}, "partial", http.StatusOK},
		{"header only", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusAccepted)
			panic("after header")
		}, "", http.StatusAccepted},
		{"flushed", func(w http.ResponseWriter, _ *http.Request) {
			_ = http.NewResponseController(w).Flush()
			panic("after flush")
		}, "", http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			log, buf := jsonLogger()
			r := chi.NewRouter()
			r.Use(httpx.RequestID(log), httpx.AccessLog(log), httpx.Recover(log))
			r.Get("/x", tt.handler)

			rec := httptest.NewRecorder()
			var got any
			func() {
				defer func() { got = recover() }()
				r.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/x", nil))
			}()

			if err, ok := got.(error); !ok || !errors.Is(err, http.ErrAbortHandler) {
				t.Fatalf("recovered %v, want http.ErrAbortHandler", got)
			}
			if rec.Body.String() != tt.wantBody || strings.Contains(rec.Body.String(), "INTERNAL") {
				t.Errorf("body = %q, want %q", rec.Body.String(), tt.wantBody)
			}
			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			var errLine, accessLine map[string]any
			for _, l := range decodeLines(t, buf.String()) {
				switch l["msg"] {
				case "panic recovered":
					errLine = l
				case "request":
					accessLine = l
				}
			}
			if errLine == nil || errLine["level"] != "ERROR" || errLine["response_started"] != true {
				t.Errorf("error log = %v", errLine)
			}
			if st, _ := errLine["stack"].(string); !strings.Contains(st, "recover_test.go") {
				t.Errorf("stack missing: %v", errLine["stack"])
			}
			if accessLine == nil || accessLine["route"] != "/x" || accessLine["status"] != float64(tt.wantStatus) {
				t.Errorf("access log = %v", accessLine)
			}
		})
	}
}

// Before the response starts, the 500 body is exactly the fixed shape.
func TestRecover_BodyExactShape(t *testing.T) {
	t.Parallel()

	r, _ := recoverRouter(t, "x")
	rec := get(t, r, "/boom", map[string]string{httpx.HeaderRequestID: "rid-shape"})
	if want := `{"code":"INTERNAL","requestId":"rid-shape"}`; rec.Body.String() != want {
		t.Errorf("body = %q, want %q", rec.Body.String(), want)
	}
}
