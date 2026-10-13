package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

const sentinel = "S3NTINEL"

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func envOf(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func baseEnv() map[string]string {
	return map[string]string{
		"API_ADDR": "127.0.0.1:0",
		// Nothing listens on port 1: the dependencies are unreachable but valid.
		"DATABASE_URL": "postgres://app:" + sentinel + "@127.0.0.1:1/app?sslmode=disable",
		"REDIS_URL":    "redis://:" + sentinel + "@127.0.0.1:1/0",
		"LOG_LEVEL":    "info",
	}
}

func TestRun_UsageErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		code int
	}{
		{"no command", nil, 2},
		{"unknown command", []string{"frobnicate"}, 2},
		{"unknown flag", []string{"api", "--nope"}, 2},
		{"extra argument", []string{"api", "extra"}, 2},
		{"help", []string{"help"}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			got := run(context.Background(), tt.args, envOf(baseEnv()), &stdout, &stderr)
			if got != tt.code {
				t.Fatalf("exit code = %d, want %d (stderr=%q)", got, tt.code, stderr.String())
			}
			out := stdout.String() + stderr.String()
			if !strings.Contains(out, "Usage: app <command>") {
				t.Fatalf("usage not printed: %q", out)
			}
		})
	}
}

func TestRun_API_ConfigErrors_ExitOneNamesVariable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mut  func(map[string]string)
		want string
	}{
		{"missing DATABASE_URL", func(m map[string]string) { delete(m, "DATABASE_URL") }, "DATABASE_URL"},
		{"missing REDIS_URL", func(m map[string]string) { delete(m, "REDIS_URL") }, "REDIS_URL"},
		{"wrong DATABASE_URL", func(m map[string]string) { m["DATABASE_URL"] = "mysql://app:" + sentinel + "@h/db" }, "DATABASE_URL"},
		{"garbage REDIS_URL", func(m map[string]string) { m["REDIS_URL"] = sentinel }, "REDIS_URL"},
		{"API_ADDR without host", func(m map[string]string) { m["API_ADDR"] = ":8080" }, "API_ADDR"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			env := baseEnv()
			tt.mut(env)

			var stdout, stderr bytes.Buffer
			code := run(context.Background(), []string{"api"}, envOf(env), &stdout, &stderr)

			if code != 1 {
				t.Fatalf("exit code = %d, want 1", code)
			}
			if !strings.Contains(stderr.String(), tt.want) {
				t.Fatalf("stderr %q does not name %s", stderr.String(), tt.want)
			}
			if all := stdout.String() + stderr.String(); strings.Contains(all, sentinel) {
				t.Fatalf("output leaks a secret value: %q", all)
			}
		})
	}
}

// TestRun_API_Lifecycle starts the real server wiring against unreachable
// dependencies: liveness is up, readiness is 503 naming both, startup logs
// are masked, and cancelling the context shuts down cleanly with exit 0.
func TestRun_API_Lifecycle(t *testing.T) {
	t.Parallel()

	var stdout, stderr syncBuffer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	exit := make(chan int, 1)
	go func() { exit <- run(ctx, []string{"api"}, envOf(baseEnv()), &stdout, &stderr) }()

	addr := waitForAddr(t, &stdout)

	status, body := get(t, "http://"+addr+"/healthz")
	if status != http.StatusOK || strings.TrimSpace(body) != `{"status":"ok"}` {
		t.Fatalf("/healthz = %d %q", status, body)
	}

	status, body = get(t, "http://"+addr+"/readyz")
	if status != http.StatusServiceUnavailable || strings.TrimSpace(body) != `{"status":"unavailable","failing":["postgres","redis"]}` {
		t.Fatalf("/readyz = %d %q", status, body)
	}

	cancel() // SIGTERM equivalent
	select {
	case code := <-exit:
		if code != 0 {
			t.Fatalf("exit code = %d, want 0\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
		}
	case <-time.After(15 * time.Second):
		t.Fatal("run did not return after context cancellation")
	}

	logs := stdout.String()
	if strings.Contains(logs, sentinel) || strings.Contains(stderr.String(), sentinel) {
		t.Fatalf("logs leak a secret:\n%s", logs)
	}
	if !strings.Contains(logs, "postgres://app:***@127.0.0.1:1/app?sslmode=disable") {
		t.Errorf("startup log should show the masked DSN:\n%s", logs)
	}
	if !strings.Contains(logs, `"msg":"shutdown complete"`) {
		t.Errorf("missing shutdown complete log:\n%s", logs)
	}
	for _, line := range strings.Split(strings.TrimSpace(logs), "\n") {
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Errorf("log line is not JSON: %q", line)
		}
	}
}

func waitForAddr(t *testing.T, out *syncBuffer) string {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		for _, line := range strings.Split(out.String(), "\n") {
			var rec struct {
				Msg  string `json:"msg"`
				Addr string `json:"addr"`
			}
			if json.Unmarshal([]byte(line), &rec) == nil && rec.Msg == "listening" {
				return rec.Addr
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("server never logged \"listening\":\n%s", out.String())
	return ""
}

func get(t *testing.T, url string) (int, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, string(b)
}
