package redisx

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/nupagachi/agentic-rag/api/internal/platform/logx"
)

func TestSlogAdapter_RedactsAndStructures(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	red := logx.NewRedactor(logx.NewSecret("redis://:S3NTINEL@127.0.0.1:6379/0"))
	a := slogAdapter{log: logx.New(&buf, slog.LevelInfo, red)}

	a.Printf(context.Background(), "redis: connection pool: failed to dial %s: %v", "redis://:S3NTINEL@127.0.0.1:6379/0", "WRONGPASS S3NTINEL")

	out := buf.String()
	if strings.Contains(out, "S3NTINEL") {
		t.Fatalf("adapter leaks secret: %s", out)
	}
	if !strings.Contains(out, `"component":"go-redis"`) || !strings.Contains(out, `"level":"WARN"`) {
		t.Fatalf("adapter output not structured: %s", out)
	}
}
