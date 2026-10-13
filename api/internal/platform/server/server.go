package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// Server limits (SR-13). There is no WriteTimeout on purpose: SSE responses
// (M-later) are long-lived; handlers bound their own work with contexts.
const (
	ReadHeaderTimeout = 5 * time.Second
	ReadTimeout       = 30 * time.Second
	IdleTimeout       = 60 * time.Second
	MaxHeaderBytes    = 32 << 10 // 32 KiB
	ShutdownTimeout   = 10 * time.Second
)

// New returns an http.Server for h with hardened timeouts. Server-internal
// errors go to log at warn level.
func New(addr string, h http.Handler, log *slog.Logger) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: ReadHeaderTimeout,
		ReadTimeout:       ReadTimeout,
		IdleTimeout:       IdleTimeout,
		MaxHeaderBytes:    MaxHeaderBytes,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelWarn),
	}
}

// Serve serves srv on ln until ctx is cancelled, then shuts down gracefully:
// no new connections, in-flight requests get up to timeout to finish. It
// returns nil on a clean shutdown. If the timeout expires the remaining
// connections are closed and an error is returned.
func Serve(ctx context.Context, srv *http.Server, ln net.Listener, timeout time.Duration) error {
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()

	select {
	case err := <-errc:
		// Serve only returns on failure here; ErrServerClosed needs Shutdown first.
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
	}

	// The caller's ctx is already cancelled; the shutdown deadline must not inherit that.
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		_ = srv.Close() // force-close what is left
		return fmt.Errorf("shutdown: %w", err)
	}
	if err := <-errc; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve: %w", err)
	}
	return nil
}
