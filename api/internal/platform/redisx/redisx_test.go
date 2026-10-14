package redisx_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/nupagachi/agentic-rag/api/internal/platform/health"
	"github.com/nupagachi/agentic-rag/api/internal/platform/logx"
	"github.com/nupagachi/agentic-rag/api/internal/platform/redisx"
)

// Compile-time check: the adapter is a health.Pinger.
var _ health.Pinger = redisx.Pinger{}

func TestOpen_InvalidURL_NoLeak(t *testing.T) {
	t.Parallel()

	c, err := redisx.Open(logx.NewSecret("http://:S3NTINEL@127.0.0.1:6379"))
	if c != nil {
		_ = c.Close()
		t.Fatal("Open returned a client")
	}
	if !errors.Is(err, redisx.ErrInvalidURL) {
		t.Fatalf("err = %v, want ErrInvalidURL", err)
	}
	if strings.Contains(err.Error(), "S3NTINEL") {
		t.Fatalf("error leaks secret: %v", err)
	}
}

func TestOpen_Lazy_UnreachableServer(t *testing.T) {
	t.Parallel()

	c, err := redisx.Open(logx.NewSecret("redis://:S3NTINEL@127.0.0.1:1/0"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = c.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if err := redisx.NewPinger(c).Ping(ctx); err == nil {
		t.Fatal("Ping succeeded against a closed port")
	}
}
