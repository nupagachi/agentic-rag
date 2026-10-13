package server_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nupagachi/agentic-rag/api/internal/platform/health"
	"github.com/nupagachi/agentic-rag/api/internal/platform/server"
)

func discardLogger() *slog.Logger { return slog.New(slog.DiscardHandler) }

func TestNewServer_Timeouts(t *testing.T) {
	t.Parallel()

	srv := server.New("127.0.0.1:8080", http.NewServeMux(), discardLogger())

	if srv.Addr != "127.0.0.1:8080" {
		t.Errorf("Addr = %q", srv.Addr)
	}
	if srv.ReadHeaderTimeout <= 0 || srv.ReadHeaderTimeout > 10*time.Second {
		t.Errorf("ReadHeaderTimeout = %v, want (0, 10s]", srv.ReadHeaderTimeout)
	}
	if srv.ReadTimeout <= 0 {
		t.Errorf("ReadTimeout = %v, want > 0", srv.ReadTimeout)
	}
	if srv.IdleTimeout <= 0 {
		t.Errorf("IdleTimeout = %v, want > 0", srv.IdleTimeout)
	}
	if srv.MaxHeaderBytes <= 0 || srv.MaxHeaderBytes > 64<<10 {
		t.Errorf("MaxHeaderBytes = %d, want (0, 64KiB]", srv.MaxHeaderBytes)
	}
	if srv.ErrorLog == nil {
		t.Error("ErrorLog must route through slog")
	}
}

func TestNewRouter_ProbesAndMiddlewares(t *testing.T) {
	t.Parallel()

	var order []string
	mw := func(name string) server.Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, name)
				w.Header().Add("X-Seen", name)
				next.ServeHTTP(w, r)
			})
		}
	}
	probes := health.NewChecker(nil, time.Second) // no dependencies: always ready
	r := server.NewRouter(server.Options{Middlewares: []server.Middleware{mw("a"), mw("b")}}, probes)

	tests := []struct {
		method string
		path   string
		status int
	}{
		{http.MethodGet, "/healthz", http.StatusOK},
		{http.MethodGet, "/readyz", http.StatusOK},
		{http.MethodGet, "/nope", http.StatusNotFound},
		{http.MethodPost, "/healthz", http.StatusMethodNotAllowed},
	}
	for _, tt := range tests {
		order = nil
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), tt.method, tt.path, nil))
		if rec.Code != tt.status {
			t.Errorf("%s %s = %d, want %d", tt.method, tt.path, rec.Code, tt.status)
		}
		if got := strings.Join(order, ","); got != "a,b" {
			t.Errorf("%s %s middleware order = %q, want a,b", tt.method, tt.path, got)
		}
	}
}

func TestNewRouter_NoMiddlewares(t *testing.T) {
	t.Parallel()

	r := server.NewRouter(server.Options{}, health.NewChecker(nil, time.Second))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
}

// fetch GETs url and returns the body. It gives up after 5 seconds.
func fetch(url string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	return string(b), err
}

func listen(t *testing.T) net.Listener {
	t.Helper()
	ln, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	return ln
}

func TestServe_GracefulShutdown_FinishesInflight(t *testing.T) {
	t.Parallel()

	inHandler := make(chan struct{})
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(inHandler)
		time.Sleep(200 * time.Millisecond)
		_, _ = io.WriteString(w, "done")
	})
	ln := listen(t)
	srv := server.New(ln.Addr().String(), h, discardLogger())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	served := make(chan error, 1)
	go func() { served <- server.Serve(ctx, srv, ln, 5*time.Second) }()

	type reply struct {
		body string
		err  error
	}
	got := make(chan reply, 1)
	go func() {
		body, err := fetch("http://" + ln.Addr().String() + "/")
		got <- reply{body: body, err: err}
	}()

	<-inHandler
	cancel() // simulate SIGTERM while the request is in flight

	r := <-got
	if r.err != nil || r.body != "done" {
		t.Fatalf("in-flight request was not completed: body=%q err=%v", r.body, r.err)
	}
	if err := <-served; err != nil {
		t.Fatalf("Serve returned %v, want nil", err)
	}

	// The listener is closed: new connections must fail.
	if c, err := (&net.Dialer{Timeout: 200 * time.Millisecond}).DialContext(t.Context(), "tcp", ln.Addr().String()); err == nil {
		_ = c.Close()
		t.Fatal("server still accepts connections after shutdown")
	}
}

func TestServe_ShutdownTimeout_ReturnsError(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	inHandler := make(chan struct{})
	h := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		close(inHandler)
		<-release
	})
	ln := listen(t)
	srv := server.New(ln.Addr().String(), h, discardLogger())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	served := make(chan error, 1)
	go func() { served <- server.Serve(ctx, srv, ln, 100*time.Millisecond) }()

	go func() { _, _ = fetch("http://" + ln.Addr().String() + "/") }()

	<-inHandler
	cancel()

	select {
	case err := <-served:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Serve returned %v, want deadline exceeded", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return after the shutdown timeout")
	}
}

func TestServe_ListenerFailure_ReturnsError(t *testing.T) {
	t.Parallel()

	ln := listen(t)
	_ = ln.Close() // Serve on a closed listener fails immediately
	srv := server.New("127.0.0.1:0", http.NewServeMux(), discardLogger())

	err := server.Serve(context.Background(), srv, ln, time.Second)
	if err == nil {
		t.Fatal("want error")
	}
}
