package db_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/nupagachi/agentic-rag/api/internal/platform/db"
	"github.com/nupagachi/agentic-rag/api/internal/platform/logx"
)

func TestOpen_InvalidDSN_NoLeak(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{
		"postgres://app:S3NTINEL@127.0.0.1:notaport/app",
		"host=localhost port=abc password=S3NTINEL",
	} {
		pool, err := db.Open(context.Background(), logx.NewSecret(raw))
		if pool != nil {
			pool.Close()
			t.Fatalf("Open(%q) returned a pool", raw)
		}
		if !errors.Is(err, db.ErrInvalidDSN) {
			t.Fatalf("err = %v, want ErrInvalidDSN", err)
		}
		if strings.Contains(err.Error(), "S3NTINEL") {
			t.Fatalf("error leaks secret: %v", err)
		}
	}
}

func TestOpen_Lazy_UnreachableDatabase(t *testing.T) {
	t.Parallel()

	// Port 1 is not listening. Open must still succeed; Ping must fail.
	pool, err := db.Open(context.Background(), logx.NewSecret("postgres://app:S3NTINEL@127.0.0.1:1/app?sslmode=disable"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if err := pool.Ping(ctx); err == nil {
		t.Fatal("Ping succeeded against a closed port")
	}
}
