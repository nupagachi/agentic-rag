package httpx_test

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/nupagachi/agentic-rag/api/internal/platform/httpx"
)

var uuidRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// jsonLogger returns a JSON logger and the buffer it writes to.
func jsonLogger() (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	return slog.New(slog.NewJSONHandler(&buf, nil)), &buf
}

func get(t *testing.T, h http.Handler, target string, hdr map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestRequestID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		header   string
		wantSame bool
	}{
		{"valid kept", "abc-123_DEF.456", true},
		{"max length kept", strings.Repeat("a", 64), true},
		{"empty replaced", "", false},
		{"too long replaced", strings.Repeat("a", 65), false},
		{"1000 chars replaced", strings.Repeat("a", 1000), false},
		{"script replaced", "<script>alert(1)</script>", false},
		{"space replaced", "abc def", false},
		{"slash replaced", "a/b", false},
		{"unicode replaced", "id-é", false},
		{"trailing newline replaced", "abc\n", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			log, buf := jsonLogger()
			var ctxID string
			var ctxLogger *slog.Logger
			r := chi.NewRouter()
			r.Use(httpx.RequestID(log))
			r.Get("/x", func(w http.ResponseWriter, r *http.Request) {
				ctxID = httpx.RequestIDFromContext(r.Context())
				ctxLogger = httpx.LoggerFromContext(r.Context())
				w.WriteHeader(http.StatusNoContent)
			})

			hdr := map[string]string{}
			if tt.header != "" {
				hdr[httpx.HeaderRequestID] = tt.header
			}
			rec := get(t, r, "/x", hdr)

			got := rec.Header().Get(httpx.HeaderRequestID)
			if tt.wantSame {
				if got != tt.header {
					t.Fatalf("response id = %q, want %q", got, tt.header)
				}
			} else if !uuidRe.MatchString(got) {
				t.Fatalf("response id = %q, want a generated UUID", got)
			}
			if ctxID != got {
				t.Errorf("context id = %q, want %q", ctxID, got)
			}
			ctxLogger.Info("hello")
			if !strings.Contains(buf.String(), `"request_id":"`+got+`"`) {
				t.Errorf("context logger line lacks request_id %q: %s", got, buf.String())
			}
		})
	}
}

func TestRequestID_CRLFNeverReachesHeaderOrLog(t *testing.T) {
	t.Parallel()

	log, buf := jsonLogger()
	r := chi.NewRouter()
	r.Use(httpx.RequestID(log))
	r.Get("/x", func(_ http.ResponseWriter, r *http.Request) {
		httpx.LoggerFromContext(r.Context()).Info("hit")
	})

	// net/http refuses to send CR/LF in a header, so a direct handler call with
	// a hand-built header is the only way to exercise this input.
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/x", nil)
	req.Header[httpx.HeaderRequestID] = []string{"good\r\nSet-Cookie: pwn=1"}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	got := rec.Header().Get(httpx.HeaderRequestID)
	if !uuidRe.MatchString(got) {
		t.Fatalf("id = %q, want generated UUID", got)
	}
	if rec.Header().Get("Set-Cookie") != "" {
		t.Error("header injection succeeded")
	}
	if strings.Contains(buf.String(), "pwn") {
		t.Errorf("log contains injected value: %s", buf.String())
	}
}

func TestRequestID_GeneratedIDsAreUnique(t *testing.T) {
	t.Parallel()

	log, _ := jsonLogger()
	h := httpx.RequestID(log)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	seen := map[string]bool{}
	for range 100 {
		id := get(t, h, "/", nil).Header().Get(httpx.HeaderRequestID)
		if seen[id] {
			t.Fatalf("duplicate id %q", id)
		}
		seen[id] = true
	}
}

func TestRequestIDFromContext_Absent(t *testing.T) {
	t.Parallel()

	if got := httpx.RequestIDFromContext(t.Context()); got != "" {
		t.Errorf("RequestIDFromContext = %q, want empty", got)
	}
	if httpx.LoggerFromContext(t.Context()) == nil {
		t.Error("LoggerFromContext must never return nil")
	}
}
