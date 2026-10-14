//go:build integration

package integration

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/nupagachi/agentic-rag/api/internal/platform/server"
)

// AC3: when shutdown starts while a request is in flight, the server stops
// accepting new connections, lets that request finish with its full response,
// and returns within the shutdown budget. Needs no stack.
//
// The binary itself has no slow route, so this drives the real server package
// (server.New and server.Serve, the same code `app api` uses) with a test
// handler. Signalling the binary is not portable (Windows cannot send SIGTERM
// to a child), which is why shutdown is triggered by cancelling the context
// that signal.NotifyContext cancels in production.
func TestServe_ShutdownLetsInFlightRequestFinish(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		close(started)
		select {
		case <-release:
		case <-r.Context().Done():
		}
		_, _ = io.WriteString(w, "finished")
	})

	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	srv := server.New(addr, mux, slog.New(slog.DiscardHandler))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	began := time.Now()
	served := make(chan error, 1)
	go func() { served <- server.Serve(ctx, srv, ln, server.ShutdownTimeout) }()

	type response struct {
		status int
		body   string
		err    error
	}
	got := make(chan response, 1)
	go func() {
		reqCtx, reqCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer reqCancel()
		status, body, err := httpGet(reqCtx, "http://"+addr+"/slow")
		got <- response{status, string(body), err}
	}()

	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("slow request never reached the handler")
	}

	cancel() // what SIGTERM does in production

	// Shutdown has begun once the listener stops accepting connections, while
	// the slow request is still held open by the handler.
	eventually(t, 5*time.Second, "listener closed after shutdown started", func() (bool, string) {
		dialer := net.Dialer{Timeout: time.Second}
		conn, err := dialer.DialContext(context.Background(), "tcp", addr)
		if err != nil {
			return true, ""
		}
		_ = conn.Close()
		return false, "still accepting connections"
	})
	select {
	case err := <-served:
		t.Fatalf("Serve returned (%v) while a request was still in flight", err)
	default:
	}

	close(release)

	r := <-got
	if r.err != nil {
		t.Fatalf("in-flight request failed during shutdown: %v", r.err)
	}
	if r.status != http.StatusOK || r.body != "finished" {
		t.Errorf("in-flight response = %d %q, want 200 \"finished\"", r.status, r.body)
	}

	select {
	case err := <-served:
		if err != nil {
			t.Errorf("Serve returned %v, want nil after a clean shutdown", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Serve did not return within 10s after the request finished")
	}
	if elapsed := time.Since(began); elapsed > 10*time.Second {
		t.Errorf("shutdown took %s, want <= 10s", elapsed)
	}
}
