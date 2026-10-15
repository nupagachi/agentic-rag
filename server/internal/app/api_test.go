package app

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/nhucanh/agentic-rag/server/internal/platform/config"
	"github.com/nhucanh/agentic-rag/server/internal/platform/lifecycle"
)

type harness struct {
	url    string
	lc     *lifecycle.Lifecycle
	cancel context.CancelFunc
	done   chan error
	closed chan struct{} // đóng khi hook "db" chạy
}

func start(t *testing.T, cfg config.HTTP, handler http.HandlerFunc) *harness {
	t.Helper()
	log := logrus.New()
	log.SetOutput(io.Discard)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{
		url:    "http://" + ln.Addr().String(),
		lc:     lifecycle.New(log),
		done:   make(chan error, 1),
		closed: make(chan struct{}),
	}
	h.lc.OnStop("db", func(context.Context) error { close(h.closed); return nil })

	ctx, cancel := context.WithCancel(t.Context())
	h.cancel = cancel
	srv := &http.Server{Handler: handler}
	go func() { h.done <- serve(ctx, srv, ln, h.lc, cfg, log) }()
	return h
}

func (h *harness) wait(t *testing.T, within time.Duration) error {
	t.Helper()
	select {
	case err := <-h.done:
		return err
	case <-time.After(within):
		t.Fatalf("serve did not return within %s", within)
		return nil
	}
}

func TestServeWaitsForInFlightRequest(t *testing.T) {
	inHandler := make(chan struct{})
	h := start(t, config.HTTP{ShutdownTimeout: 5 * time.Second}, func(w http.ResponseWriter, r *http.Request) {
		close(inHandler)
		time.Sleep(300 * time.Millisecond)
		_, _ = w.Write([]byte("done"))
	})

	type result struct {
		status int
		err    error
	}
	res := make(chan result, 1)
	go func() {
		resp, err := http.Get(h.url)
		if err != nil {
			res <- result{err: err}
			return
		}
		_ = resp.Body.Close()
		res <- result{status: resp.StatusCode}
	}()

	<-inHandler
	h.cancel() // tương đương nhận SIGTERM

	if err := h.wait(t, 3*time.Second); err != nil {
		t.Fatalf("serve: %v", err)
	}
	r := <-res
	if r.err != nil || r.status != http.StatusOK {
		t.Fatalf("in-flight request: status=%d err=%v, want 200", r.status, r.err)
	}
	select {
	case <-h.closed:
	default:
		t.Fatal("resources not closed")
	}
	if _, err := http.Get(h.url); err == nil {
		t.Fatal("server still accepts connections after shutdown")
	}
}

func TestServeMarksShuttingDownDuringDrain(t *testing.T) {
	h := start(t, config.HTTP{ShutdownTimeout: time.Second, DrainDelay: 300 * time.Millisecond},
		func(w http.ResponseWriter, r *http.Request) {})

	h.cancel()
	deadline := time.Now().Add(time.Second)
	for !h.lc.ShuttingDown() {
		if time.Now().After(deadline) {
			t.Fatal("lifecycle not marked shutting down")
		}
		time.Sleep(5 * time.Millisecond)
	}
	// Trong drain delay listener vẫn mở: request vẫn được phục vụ.
	resp, err := http.Get(h.url)
	if err != nil {
		t.Fatalf("request during drain: %v", err)
	}
	_ = resp.Body.Close()

	if err := h.wait(t, 3*time.Second); err != nil {
		t.Fatalf("serve: %v", err)
	}
}

func TestServeForcesCloseAfterTimeout(t *testing.T) {
	inHandler := make(chan struct{})
	release := make(chan struct{})
	defer close(release)
	h := start(t, config.HTTP{ShutdownTimeout: 200 * time.Millisecond}, func(w http.ResponseWriter, r *http.Request) {
		close(inHandler)
		<-release // request treo
	})

	go func() {
		if resp, err := http.Get(h.url); err == nil {
			_ = resp.Body.Close()
		}
	}()
	<-inHandler
	h.cancel()

	err := h.wait(t, 3*time.Second)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want DeadlineExceeded", err)
	}
	select {
	case <-h.closed:
	default:
		t.Fatal("resources not closed after forced shutdown")
	}
}

func TestStreamEndsOnShutdown(t *testing.T) {
	// Handler kiểu SSE: chạy tới khi client ngắt hoặc server bắt đầu tắt.
	var lc *lifecycle.Lifecycle
	started := make(chan struct{})
	h := start(t, config.HTTP{ShutdownTimeout: 5 * time.Second}, func(w http.ResponseWriter, r *http.Request) {
		close(started)
		select {
		case <-r.Context().Done():
		case <-lc.Context().Done():
		}
	})
	lc = h.lc

	go func() {
		if resp, err := http.Get(h.url); err == nil {
			_ = resp.Body.Close()
		}
	}()
	<-started
	begin := time.Now()
	h.cancel()

	if err := h.wait(t, 3*time.Second); err != nil {
		t.Fatalf("serve: %v", err)
	}
	if d := time.Since(begin); d > time.Second {
		t.Fatalf("shutdown took %s, stream did not end promptly", d)
	}
}
