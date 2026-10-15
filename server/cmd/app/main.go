// Một binary, nhiều vai trò: app api | healthcheck | scheduler | eval-runner | migrate.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/nhucanh/agentic-rag/server/internal/app"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// run tách khỏi main để defer (dừng nhận tín hiệu) chạy trước os.Exit.
func run() error {
	cmd := "api"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	ctx, stop := signalContext()
	defer stop()

	switch cmd {
	case "api":
		return app.RunAPI(ctx)
	case "healthcheck":
		return app.Healthcheck()
	default:
		return fmt.Errorf("unknown command %q (available: api, healthcheck)", cmd)
	}
}

// signalContext hủy ctx ở tín hiệu SIGINT/SIGTERM đầu tiên để bắt đầu tắt êm;
// tín hiệu thứ hai thoát ngay (exit 130) khi không muốn chờ.
func signalContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	sig := make(chan os.Signal, 2)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		s := <-sig
		fmt.Fprintf(os.Stderr, "received %s, shutting down gracefully (send again to force)\n", s)
		cancel()
		s = <-sig
		fmt.Fprintf(os.Stderr, "received %s again, forcing exit\n", s)
		os.Exit(130)
	}()

	return ctx, func() { signal.Stop(sig); cancel() }
}
