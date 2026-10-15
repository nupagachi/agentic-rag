package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"github.com/nhucanh/agentic-rag/server/internal/platform/config"
)

func init() { gin.SetMode(gin.TestMode) }

func newRouter(t *testing.T, cfg config.HTTP, register func(r *gin.Engine)) *gin.Engine {
	t.Helper()
	log := logrus.New()
	log.SetOutput(io.Discard)
	r, err := NewRouter(RouterParams{
		Config: &config.Config{HTTP: cfg},
		Log:    log,
		Routes: []Routes{routesFunc(register)},
	})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

type routesFunc func(r *gin.Engine)

func (f routesFunc) Register(r *gin.Engine) { f(r) }

type errorResponse struct {
	Error struct {
		Code      string         `json:"code"`
		Details   map[string]any `json:"details"`
		RequestID string         `json:"request_id"`
	} `json:"error"`
}

func do(t *testing.T, r http.Handler, req *http.Request) (*httptest.ResponseRecorder, errorResponse) {
	t.Helper()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var body errorResponse
	if w.Code >= 400 && w.Body.Len() > 0 {
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode error body %q: %v", w.Body.String(), err)
		}
	}
	return w, body
}

func TestRequestIDKeepsValidAndReplacesUnsafe(t *testing.T) {
	r := newRouter(t, config.HTTP{}, func(r *gin.Engine) {
		r.GET("/x", func(c *gin.Context) { c.String(http.StatusOK, RequestIDFrom(c)) })
	})

	tests := []struct {
		in       string
		keep     bool
		testName string
	}{
		{"abc-123_DEF.4:5", true, "safe"},
		{"", false, "empty"},
		{"evil\nlevel=error", false, "newline"},
		{"has space", false, "space"},
		{strings.Repeat("a", maxRequestIDLen+1), false, "too long"},
	}
	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/x", nil)
			req.Header.Set(HeaderRequestID, tt.in)
			w, _ := do(t, r, req)
			got := w.Header().Get(HeaderRequestID)
			if got != w.Body.String() {
				t.Fatalf("header %q != context %q", got, w.Body.String())
			}
			if tt.keep != (got == tt.in) {
				t.Fatalf("in=%q got=%q keep=%v", tt.in, got, tt.keep)
			}
		})
	}
}

func TestErrorHandlerRendersAppErrorAndHidesInternal(t *testing.T) {
	r := newRouter(t, config.HTTP{}, func(r *gin.Engine) {
		r.GET("/conflict", func(c *gin.Context) { Error(c, ErrConflict) })
		r.GET("/boom", func(c *gin.Context) { Error(c, errors.New("db password=secret")) })
	})

	w, body := do(t, r, httptest.NewRequest(http.MethodGet, "/conflict", nil))
	if w.Code != http.StatusConflict || body.Error.Code != "conflict" || body.Error.RequestID == "" {
		t.Fatalf("conflict: %d %+v", w.Code, body)
	}

	w, body = do(t, r, httptest.NewRequest(http.MethodGet, "/boom", nil))
	if w.Code != http.StatusInternalServerError || body.Error.Code != "internal_error" {
		t.Fatalf("boom: %d %+v", w.Code, body)
	}
	if strings.Contains(w.Body.String(), "secret") {
		t.Fatalf("internal error leaked to client: %s", w.Body.String())
	}
}

func TestRecovery(t *testing.T) {
	r := newRouter(t, config.HTTP{}, func(r *gin.Engine) {
		r.GET("/panic", func(*gin.Context) { panic("boom") })
		r.GET("/panic-after-write", func(c *gin.Context) {
			c.String(http.StatusOK, "partial")
			panic("boom")
		})
	})

	w, body := do(t, r, httptest.NewRequest(http.MethodGet, "/panic", nil))
	if w.Code != http.StatusInternalServerError || body.Error.Code != "internal_error" {
		t.Fatalf("panic: %d %+v", w.Code, body)
	}

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/panic-after-write", nil))
	if w.Code != http.StatusOK || w.Body.String() != "partial" {
		t.Fatalf("response already sent must not be rewritten: %d %q", w.Code, w.Body.String())
	}
}

func TestRecoveryRepanicsAbortHandler(t *testing.T) {
	r := newRouter(t, config.HTTP{}, func(r *gin.Engine) {
		r.GET("/abort", func(*gin.Context) { panic(http.ErrAbortHandler) })
	})
	defer func() {
		if rec := recover(); rec != http.ErrAbortHandler { //nolint:errorlint // so sánh giá trị panic
			t.Fatalf("recovered %v, want http.ErrAbortHandler", rec)
		}
	}()
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/abort", nil))
}

func TestNoRouteAndNoMethod(t *testing.T) {
	r := newRouter(t, config.HTTP{}, func(r *gin.Engine) {
		r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })
	})
	if w, body := do(t, r, httptest.NewRequest(http.MethodGet, "/missing", nil)); w.Code != 404 || body.Error.Code != "not_found" {
		t.Fatalf("no route: %d %+v", w.Code, body)
	}
	if w, body := do(t, r, httptest.NewRequest(http.MethodPost, "/x", nil)); w.Code != 405 || body.Error.Code != "method_not_allowed" {
		t.Fatalf("no method: %d %+v", w.Code, body)
	}
}

func TestMaxBodyBytes(t *testing.T) {
	r := newRouter(t, config.HTTP{MaxBodyBytes: 16}, func(r *gin.Engine) {
		r.POST("/read", func(c *gin.Context) {
			if _, err := io.ReadAll(c.Request.Body); err != nil {
				Error(c, err)
				return
			}
			c.Status(http.StatusNoContent)
		})
	})

	small := httptest.NewRequest(http.MethodPost, "/read", strings.NewReader("ok"))
	if w, _ := do(t, r, small); w.Code != http.StatusNoContent {
		t.Fatalf("small body: %d", w.Code)
	}

	big := httptest.NewRequest(http.MethodPost, "/read", strings.NewReader(strings.Repeat("x", 64)))
	if w, body := do(t, r, big); w.Code != http.StatusRequestEntityTooLarge || body.Error.Code != "payload_too_large" {
		t.Fatalf("content-length over limit: %d %+v", w.Code, body)
	}

	chunked := httptest.NewRequest(http.MethodPost, "/read", strings.NewReader(strings.Repeat("x", 64)))
	chunked.ContentLength = -1 // không biết trước độ dài: chặn lúc đọc
	if w, body := do(t, r, chunked); w.Code != http.StatusRequestEntityTooLarge || body.Error.Code != "payload_too_large" {
		t.Fatalf("chunked over limit: %d %+v", w.Code, body)
	}
}
