package health

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"github.com/nhucanh/agentic-rag/server/internal/platform/config"
	"github.com/nhucanh/agentic-rag/server/internal/platform/lifecycle"
)

type readiness struct {
	Status string `json:"status"`
	Checks map[string]struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	} `json:"checks"`
}

func setup(env string, checks ...Check) (*gin.Engine, *lifecycle.Lifecycle) {
	gin.SetMode(gin.TestMode)
	log := logrus.New()
	log.SetOutput(io.Discard)
	lc := lifecycle.New(log)
	r := gin.New()
	NewHandler(Params{Config: &config.Config{Env: env}, Lifecycle: lc, Checks: checks}).Register(r)
	return r, lc
}

func get(t *testing.T, r http.Handler, path string) (int, readiness) {
	t.Helper()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	var body readiness
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode %q: %v", w.Body.String(), err)
	}
	return w.Code, body
}

func up(name string) Check {
	return Check{Name: name, Ping: func(context.Context) error { return nil }}
}

func down(name string) Check {
	return Check{Name: name, Ping: func(context.Context) error { return errors.New("dial tcp: refused") }}
}

func TestReadyAllUp(t *testing.T) {
	r, _ := setup(config.EnvDevelopment, up("postgres"), up("redis"))
	code, body := get(t, r, "/readyz")
	if code != http.StatusOK || body.Status != "ok" || len(body.Checks) != 2 {
		t.Fatalf("got %d %+v", code, body)
	}
}

func TestReadyOneDown(t *testing.T) {
	r, _ := setup(config.EnvDevelopment, up("postgres"), down("redis"))
	code, body := get(t, r, "/readyz")
	if code != http.StatusServiceUnavailable || body.Status != "unavailable" {
		t.Fatalf("got %d %+v", code, body)
	}
	if c := body.Checks["redis"]; c.Status != "down" || c.Error == "" {
		t.Fatalf("redis = %+v, want down with error in development", c)
	}
}

func TestReadyHidesErrorsInProduction(t *testing.T) {
	r, _ := setup(config.EnvProduction, down("redis"))
	_, body := get(t, r, "/readyz")
	if c := body.Checks["redis"]; c.Status != "down" || c.Error != "" {
		t.Fatalf("redis = %+v, want down without error detail", c)
	}
}

func TestReadyShuttingDown(t *testing.T) {
	r, lc := setup(config.EnvDevelopment, up("postgres"))
	lc.Shutdown()
	code, body := get(t, r, "/readyz")
	if code != http.StatusServiceUnavailable || body.Status != "shutting_down" {
		t.Fatalf("got %d %+v", code, body)
	}
	// Liveness không đổi khi đang tắt: Docker không được restart container giữa chừng.
	if code, body := get(t, r, "/healthz"); code != http.StatusOK || body.Status != "ok" {
		t.Fatalf("healthz: %d %+v", code, body)
	}
}
