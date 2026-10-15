package app

import (
	"io"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"

	"github.com/nhucanh/agentic-rag/server/internal/apidocs"
	"github.com/nhucanh/agentic-rag/server/internal/health"
	"github.com/nhucanh/agentic-rag/server/internal/platform/cache"
	"github.com/nhucanh/agentic-rag/server/internal/platform/config"
	"github.com/nhucanh/agentic-rag/server/internal/platform/database"
	"github.com/nhucanh/agentic-rag/server/internal/platform/httpx"
	"github.com/nhucanh/agentic-rag/server/internal/platform/lifecycle"
)

var ginParam = regexp.MustCompile(`[:*](\w+)`)

// TestRoutesMatchSpec bảo đảm mọi route Gin có trong contracts/openapi.yaml và
// ngược lại, để tài liệu FE đọc không lệch với server.
func TestRoutesMatchSpec(t *testing.T) {
	log := logrus.New()
	log.SetOutput(io.Discard)
	cfg := &config.Config{
		Postgres: config.Postgres{DSN: "postgres://x@127.0.0.1:1/x"},
		APIDocs:  config.APIDocs{Enabled: true},
	}
	lc := lifecycle.New(log)
	db, err := database.New(cfg, log, lc)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lc.Stop(t.Context()) })

	r, err := httpx.NewRouter(httpx.RouterParams{Config: cfg, Log: log, Routes: []httpx.Routes{
		health.NewHandler(health.Params{Config: cfg, Lifecycle: lc, Checks: []health.Check{
			postgresCheck(db), redisCheck(cache.New(cfg, log, lc)),
		}}),
		apidocs.NewHandler(cfg),
	}})
	if err != nil {
		t.Fatal(err)
	}

	var served []string
	for _, rt := range r.Routes() {
		if rt.Path == apidocs.UIPath || strings.HasPrefix(rt.Path, apidocs.UIPath+"/") {
			continue // trang Swagger UI, không phải API
		}
		served = append(served, rt.Method+" "+ginParam.ReplaceAllString(rt.Path, "{$1}"))
	}

	spec, err := apidocs.LoadSpec()
	if err != nil {
		t.Fatal(err)
	}
	var documented []string
	for path, item := range spec.Paths.Map() {
		for method := range item.Operations() {
			documented = append(documented, method+" "+path)
		}
	}

	for _, s := range served {
		if !slices.Contains(documented, s) {
			t.Errorf("route %q chưa có trong contracts/openapi.yaml", s)
		}
	}
	for _, d := range documented {
		if !slices.Contains(served, d) {
			t.Errorf("spec có %q nhưng server chưa đăng ký route", d)
		}
	}
}
