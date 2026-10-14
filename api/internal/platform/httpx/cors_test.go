package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/nupagachi/agentic-rag/api/internal/platform/httpx"
)

func corsRouter(origins []string) (*chi.Mux, *int) {
	hits := new(int)
	r := chi.NewRouter()
	r.Use(httpx.CORS(origins))
	r.Get("/x", func(w http.ResponseWriter, _ *http.Request) {
		*hits++
		_, _ = w.Write([]byte("ok"))
	})
	return r, hits
}

func do(t *testing.T, h http.Handler, method, target string, hdr map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), method, target, nil)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestCORS_SimpleRequest(t *testing.T) {
	t.Parallel()

	allow := []string{"http://localhost:3000", "https://app.example.com"}
	tests := []struct {
		name      string
		origin    string
		wantAllow string
	}{
		{"allowed origin echoed", "http://localhost:3000", "http://localhost:3000"},
		{"second allowed origin", "https://app.example.com", "https://app.example.com"},
		{"unknown origin", "https://evil.example", ""},
		{"null origin", "null", ""},
		{"no origin header", "", ""},
		{"case differs", "HTTP://LOCALHOST:3000", ""},
		{"suffix trick", "http://localhost:3000.evil.example", ""},
		{"port differs", "http://localhost:3001", ""},
		{"scheme differs", "https://localhost:3000", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r, hits := corsRouter(allow)
			hdr := map[string]string{}
			if tt.origin != "" {
				hdr["Origin"] = tt.origin
			}
			rec := do(t, r, http.MethodGet, "/x", hdr)

			if rec.Code != http.StatusOK || *hits != 1 {
				t.Fatalf("status = %d, hits = %d", rec.Code, *hits)
			}
			if got := rec.Header().Get("Access-Control-Allow-Origin"); got != tt.wantAllow {
				t.Errorf("Allow-Origin = %q, want %q", got, tt.wantAllow)
			}
			if tt.wantAllow != "" {
				if got := rec.Header().Get("Access-Control-Expose-Headers"); got != "X-Request-ID" {
					t.Errorf("Expose-Headers = %q", got)
				}
			} else if rec.Header().Get("Access-Control-Expose-Headers") != "" {
				t.Error("Expose-Headers set for a disallowed origin")
			}
			if !strings.Contains(strings.Join(rec.Header().Values("Vary"), ","), "Origin") {
				t.Errorf("Vary = %v, want Origin", rec.Header().Values("Vary"))
			}
			if _, ok := rec.Header()["Access-Control-Allow-Credentials"]; ok {
				t.Error("Allow-Credentials must never be set")
			}
		})
	}
}

func TestCORS_Preflight(t *testing.T) {
	t.Parallel()

	allow := []string{"http://localhost:3000"}
	tests := []struct {
		name       string
		origin     string
		reqMethod  string
		wantOrigin string
	}{
		{"allowed GET", "http://localhost:3000", "GET", "http://localhost:3000"},
		{"allowed HEAD", "http://localhost:3000", "HEAD", "http://localhost:3000"},
		{"POST not granted", "http://localhost:3000", "POST", ""},
		{"DELETE not granted", "http://localhost:3000", "DELETE", ""},
		{"PUT not granted", "http://localhost:3000", "PUT", ""},
		{"unknown origin", "https://evil.example", "GET", ""},
		{"null origin", "null", "GET", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r, hits := corsRouter(allow)
			rec := do(t, r, http.MethodOptions, "/x", map[string]string{
				"Origin":                         tt.origin,
				"Access-Control-Request-Method":  tt.reqMethod,
				"Access-Control-Request-Headers": "authorization",
			})

			if rec.Code != http.StatusNoContent {
				t.Errorf("status = %d, want 204", rec.Code)
			}
			if *hits != 0 {
				t.Error("preflight must not reach the router")
			}
			h := rec.Header()
			if got := h.Get("Access-Control-Allow-Origin"); got != tt.wantOrigin {
				t.Errorf("Allow-Origin = %q, want %q", got, tt.wantOrigin)
			}
			if tt.wantOrigin == "" {
				for _, k := range []string{"Access-Control-Allow-Methods", "Access-Control-Allow-Headers", "Access-Control-Max-Age"} {
					if h.Get(k) != "" {
						t.Errorf("%s set for a refused preflight", k)
					}
				}
				return
			}
			if got := h.Get("Access-Control-Allow-Methods"); got != "GET, HEAD, OPTIONS" {
				t.Errorf("Allow-Methods = %q", got)
			}
			if got := h.Get("Access-Control-Allow-Headers"); got != "Content-Type, Authorization, X-Request-ID" {
				t.Errorf("Allow-Headers = %q", got)
			}
			if h.Get("Access-Control-Max-Age") == "" {
				t.Error("Max-Age missing")
			}
			vary := strings.Join(h.Values("Vary"), ",")
			for _, want := range []string{"Origin", "Access-Control-Request-Method", "Access-Control-Request-Headers"} {
				if !strings.Contains(vary, want) {
					t.Errorf("Vary %q lacks %s", vary, want)
				}
			}
			if _, ok := h["Access-Control-Allow-Credentials"]; ok {
				t.Error("Allow-Credentials must never be set")
			}
		})
	}
}

// An OPTIONS without Access-Control-Request-Method is not a preflight and
// passes through to the router.
func TestCORS_PlainOptionsPassesThrough(t *testing.T) {
	t.Parallel()

	r := chi.NewRouter()
	r.Use(httpx.CORS([]string{"http://localhost:3000"}))
	var reached bool
	r.Options("/x", func(w http.ResponseWriter, _ *http.Request) {
		reached = true
		w.WriteHeader(http.StatusNoContent)
	})
	do(t, r, http.MethodOptions, "/x", map[string]string{"Origin": "http://localhost:3000"})
	if !reached {
		t.Error("plain OPTIONS did not reach the router")
	}
}

// Wildcard and null entries in the allowlist are ignored, never honoured.
func TestCORS_NeverWildcardOrNull(t *testing.T) {
	t.Parallel()

	r, _ := corsRouter([]string{"*", "null", "", "http://localhost:3000"})
	for _, origin := range []string{"https://anything.example", "null"} {
		rec := do(t, r, http.MethodGet, "/x", map[string]string{"Origin": origin})
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("origin %q got Allow-Origin %q", origin, got)
		}
	}
	rec := do(t, r, http.MethodGet, "/x", map[string]string{"Origin": "http://localhost:3000"})
	if rec.Header().Get("Access-Control-Allow-Origin") == "*" {
		t.Error("wildcard emitted")
	}
}

func TestCORS_EmptyAllowlistGrantsNothing(t *testing.T) {
	t.Parallel()

	r, _ := corsRouter(nil)
	rec := do(t, r, http.MethodGet, "/x", map[string]string{"Origin": "http://localhost:3000"})
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Allow-Origin = %q, want empty", got)
	}
}
