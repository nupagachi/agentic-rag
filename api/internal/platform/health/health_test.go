package health_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"agentic-rag/api/internal/platform/health"
	"agentic-rag/api/internal/platform/logx"
)

type pingerFunc func(ctx context.Context) error

func (f pingerFunc) Ping(ctx context.Context) error { return f(ctx) }

func ok() health.Pinger { return pingerFunc(func(context.Context) error { return nil }) }

func failWith(msg string) health.Pinger {
	return pingerFunc(func(context.Context) error { return errors.New(msg) })
}

// blockUntilCtx honors the context, like pgx and go-redis do.
func blockUntilCtx() health.Pinger {
	return pingerFunc(func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})
}

func newChecker(timeout time.Duration, pg, rd health.Pinger) *health.Checker {
	return health.NewChecker(nil, timeout,
		health.Check{Name: health.Postgres, Pinger: pg},
		health.Check{Name: health.Redis, Pinger: rd},
	)
}

func TestChecker_Failing(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	// ignoresCtx never looks at the context at all.
	ignoresCtx := pingerFunc(func(context.Context) error {
		<-release
		return nil
	})

	tests := []struct {
		name string
		pg   health.Pinger
		rd   health.Pinger
		want []health.Dependency
	}{
		{"both ok", ok(), ok(), nil},
		{"postgres fails", failWith("boom"), ok(), []health.Dependency{health.Postgres}},
		{"redis fails", ok(), failWith("boom"), []health.Dependency{health.Redis}},
		{"both fail", failWith("a"), failWith("b"), []health.Dependency{health.Postgres, health.Redis}},
		{"redis times out honoring ctx", ok(), blockUntilCtx(), []health.Dependency{health.Redis}},
		{"postgres times out ignoring ctx", ignoresCtx, ok(), []health.Dependency{health.Postgres}},
		{"both time out", blockUntilCtx(), blockUntilCtx(), []health.Dependency{health.Postgres, health.Redis}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := newChecker(50*time.Millisecond, tt.pg, tt.rd)
			got := c.Failing(context.Background())
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Failing = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestChecker_RunsInParallel(t *testing.T) {
	t.Parallel()

	slow := pingerFunc(func(ctx context.Context) error {
		select {
		case <-time.After(150 * time.Millisecond):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	c := newChecker(time.Second, slow, slow)

	start := time.Now()
	if got := c.Failing(context.Background()); len(got) != 0 {
		t.Fatalf("Failing = %v, want none", got)
	}
	if elapsed := time.Since(start); elapsed >= 280*time.Millisecond {
		t.Fatalf("checks ran sequentially: %v", elapsed)
	}
}

func get(t *testing.T, h http.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	return rec
}

func TestHealthz_DoesNotCheckDependencies(t *testing.T) {
	t.Parallel()

	c := newChecker(time.Second,
		pingerFunc(func(context.Context) error { t.Error("healthz pinged postgres"); return nil }),
		pingerFunc(func(context.Context) error { t.Error("healthz pinged redis"); return nil }),
	)
	rec := get(t, c.Healthz)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != `{"status":"ok"}` {
		t.Fatalf("body = %s", got)
	}
}

func TestReadyz_OK(t *testing.T) {
	t.Parallel()

	rec := get(t, newChecker(time.Second, ok(), ok()).Readyz)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != `{"status":"ok"}` {
		t.Fatalf("body = %s", got)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content-type = %q", ct)
	}
}

func TestReadyz_Unavailable(t *testing.T) {
	t.Parallel()

	rec := get(t, newChecker(50*time.Millisecond, ok(), blockUntilCtx()).Readyz)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != `{"status":"unavailable","failing":["redis"]}` {
		t.Fatalf("body = %s", got)
	}
}

func TestReadyz_FailingBodyOnlyNames(t *testing.T) {
	t.Parallel()

	const leak = "dial tcp 10.0.0.5:5432: password=S3NTINEL refused"
	rec := get(t, newChecker(time.Second, failWith(leak), failWith(leak)).Readyz)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", rec.Code)
	}
	var got struct {
		Status  string   `json:"status"`
		Failing []string `json:"failing"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	if got.Status != "unavailable" || !reflect.DeepEqual(got.Failing, []string{"postgres", "redis"}) {
		t.Fatalf("body = %+v", got)
	}
	for _, forbidden := range []string{"S3NTINEL", "10.0.0.5", "dial", "refused"} {
		if strings.Contains(rec.Body.String(), forbidden) {
			t.Fatalf("body leaks %q: %s", forbidden, rec.Body.String())
		}
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &keys); err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 {
		t.Fatalf("body has unexpected keys: %s", rec.Body.String())
	}
}

func TestReadyz_LogsFailureWithRedaction(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	red := logx.NewRedactor(logx.NewSecret("redis://:S3NTINEL@127.0.0.1:6379/0"))
	c := health.NewChecker(logx.New(&buf, slog.LevelInfo, red), time.Second,
		health.Check{Name: health.Redis, Pinger: failWith("WRONGPASS S3NTINEL")},
	)
	if got := c.Failing(context.Background()); !reflect.DeepEqual(got, []health.Dependency{health.Redis}) {
		t.Fatalf("Failing = %v", got)
	}
	out := buf.String()
	if !strings.Contains(out, `"dependency":"redis"`) {
		t.Fatalf("failure not logged: %s", out)
	}
	if strings.Contains(out, "S3NTINEL") {
		t.Fatalf("failure log leaks secret: %s", out)
	}
}
