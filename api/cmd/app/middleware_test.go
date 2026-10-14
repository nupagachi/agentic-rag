package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

// header GETs url with the given request headers and returns the response
// status and headers.
func header(t *testing.T, url string, hdr map[string]string) (int, http.Header) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode, resp.Header
}

// TestRun_API_Middlewares proves the base middlewares are wired into the real
// binary: request id echoed, CORS default origin allowed (config default) and
// others refused, and one allowlisted access log line per request.
func TestRun_API_Middlewares(t *testing.T) {
	t.Parallel()

	var stdout, stderr syncBuffer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	exit := make(chan int, 1)
	// baseEnv sets no CORS_ALLOWED_ORIGINS, so this also covers the config default.
	go func() { exit <- run(ctx, []string{"api"}, envOf(baseEnv()), &stdout, &stderr) }()
	addr := waitForAddr(t, &stdout)
	base := "http://" + addr

	status, h := header(t, base+"/healthz?token=SENTINEL-QUERY", map[string]string{
		"X-Request-ID":  "rid-e2e",
		"Origin":        "http://localhost:3000",
		"Authorization": "Bearer SENTINEL-AUTH",
	})
	if status != http.StatusOK {
		t.Fatalf("/healthz = %d", status)
	}
	if got := h.Get("X-Request-ID"); got != "rid-e2e" {
		t.Errorf("X-Request-ID = %q, want rid-e2e", got)
	}
	if got := h.Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Errorf("Allow-Origin = %q, want the default dev origin", got)
	}

	// Without a client id one is generated; a foreign origin gets no CORS grant.
	_, h = header(t, base+"/healthz", map[string]string{"Origin": "https://evil.example"})
	if h.Get("X-Request-ID") == "" {
		t.Error("generated X-Request-ID missing")
	}
	if got := h.Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("foreign origin got Allow-Origin %q", got)
	}

	cancel()
	select {
	case code := <-exit:
		if code != 0 {
			t.Fatalf("exit code = %d\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
		}
	case <-time.After(15 * time.Second):
		t.Fatal("run did not return after context cancellation")
	}

	logs := stdout.String()
	for _, leak := range []string{"SENTINEL-QUERY", "SENTINEL-AUTH"} {
		if strings.Contains(logs, leak) {
			t.Errorf("logs leak %s:\n%s", leak, logs)
		}
	}
	var found bool
	for _, line := range strings.Split(strings.TrimSpace(logs), "\n") {
		var rec struct {
			Msg       string  `json:"msg"`
			RequestID string  `json:"request_id"`
			Method    string  `json:"method"`
			Route     string  `json:"route"`
			Status    int     `json:"status"`
			Duration  float64 `json:"duration_ms"`
		}
		if json.Unmarshal([]byte(line), &rec) != nil || rec.Msg != "request" || rec.RequestID != "rid-e2e" {
			continue
		}
		found = true
		if rec.Method != "GET" || rec.Route != "/healthz" || rec.Status != 200 || rec.Duration < 0 {
			t.Errorf("access log line = %+v", rec)
		}
	}
	if !found {
		t.Errorf("no access log line for rid-e2e:\n%s", logs)
	}
}
