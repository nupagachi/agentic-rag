//go:build integration

// Package integration holds the API integration tests of milestone M0.
//
// Decision (M0): these tests run against the dev stack from `make up`
// (Postgres on 127.0.0.1:5432, Redis on 127.0.0.1:6379) instead of
// testcontainers, to keep M0 light. Connection settings come from the
// environment (`make -C api test-integration` loads ../.env first).
//
// A test that needs the stack skips with an explicit reason when the stack is
// not configured or not reachable. Set INTEGRATION_REQUIRE_STACK=1 (CI) to turn
// those skips into failures so a missing stack can never look like a pass.
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	requireStackEnv = "INTEGRATION_REQUIRE_STACK"

	// sentinel is a fake password for URLs that point at dead ports. It must
	// never show up in a response body or in the process output.
	sentinel = "S3NTINEL"

	// deadAddr is a loopback port where nothing listens.
	deadAddr = "127.0.0.1:1"

	startupTimeout  = 15 * time.Second
	shutdownTimeout = 10 * time.Second
)

// skipOrFail skips the test, or fails it when INTEGRATION_REQUIRE_STACK=1.
func skipOrFail(t *testing.T, format string, args ...any) {
	t.Helper()
	if os.Getenv(requireStackEnv) == "1" {
		t.Fatalf(format+" ("+requireStackEnv+"=1)", args...)
	}
	t.Skipf(format, args...)
}

// stack is the connection settings of the running dev stack.
type stack struct {
	databaseURL string
	redisURL    string
}

// requireStack returns the stack settings or skips the test when DATABASE_URL
// or REDIS_URL is missing, or when the service does not accept TCP connections.
// Messages never include the URLs: they carry passwords.
func requireStack(t *testing.T) stack {
	t.Helper()
	deps := []struct {
		name, env, defaultPort string
	}{
		{"postgres", "DATABASE_URL", "5432"},
		{"redis", "REDIS_URL", "6379"},
	}
	got := map[string]string{}
	for _, d := range deps {
		raw := os.Getenv(d.env)
		if raw == "" {
			skipOrFail(t, "integration stack not configured: %s is not set; run `make up`, then `make -C api test-integration` (it loads ../.env)", d.env)
		}
		u, err := url.Parse(raw)
		if err != nil || u.Hostname() == "" {
			skipOrFail(t, "integration stack not configured: %s is not a valid URL", d.env)
		}
		port := u.Port()
		if port == "" {
			port = d.defaultPort
		}
		hostPort := net.JoinHostPort(u.Hostname(), port)
		dialer := net.Dialer{Timeout: 2 * time.Second}
		conn, err := dialer.DialContext(context.Background(), "tcp", hostPort)
		if err != nil {
			skipOrFail(t, "integration stack not running: cannot reach %s at %s; run `make up`, then `make -C api test-integration`", d.name, hostPort)
		}
		_ = conn.Close()
		got[d.name] = raw
	}
	return stack{databaseURL: got["postgres"], redisURL: got["redis"]}
}

// passwordsOf returns the password inside a URL in both forms it can take in
// output: as written in the URL (percent-encoded) and decoded. It returns nil
// when there is no password. The URL is parsed by hand because url.Parse
// decodes the userinfo and the encoded form would be lost.
func passwordsOf(raw string) []string {
	_, rest, ok := strings.Cut(raw, "://")
	if !ok {
		return nil
	}
	authority, _, _ := strings.Cut(rest, "/")
	at := strings.LastIndex(authority, "@")
	if at < 0 {
		return nil
	}
	_, encoded, ok := strings.Cut(authority[:at], ":")
	if !ok || encoded == "" {
		return nil
	}
	out := []string{encoded}
	if decoded, err := url.PathUnescape(encoded); err == nil && decoded != encoded {
		out = append(out, decoded)
	}
	return out
}

var userinfoPattern = regexp.MustCompile(`://[^/\s@]*@`)

// redact makes text safe to print in a test failure: known secrets become ***
// and any userinfo in a URL (://user:pass@) becomes ://***@.
func redact(text string, secrets ...string) string {
	for _, s := range secrets {
		if s != "" {
			text = strings.ReplaceAll(text, s, "***")
		}
	}
	return userinfoPattern.ReplaceAllString(text, "://***@")
}

// withPassword returns raw with its password replaced.
func withPassword(t *testing.T, raw, password string) string {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal("cannot parse URL to replace its password")
	}
	user := ""
	if u.User != nil {
		user = u.User.Username()
	}
	u.User = url.UserPassword(user, password)
	return u.String()
}

// syncBuffer is a bytes.Buffer safe for the exec copy goroutine.
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

var (
	buildOnce sync.Once
	binDir    string
	binPath   string
	errBuild  error
)

// TestMain removes the temporary directory that holds the built binary.
func TestMain(m *testing.M) {
	code := m.Run()
	if binDir != "" {
		_ = os.RemoveAll(binDir)
	}
	os.Exit(code)
}

// appBinary builds the real `app` binary once per test run.
func appBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "app-integration-")
		if err != nil {
			errBuild = err
			return
		}
		name := "app"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		binDir = dir
		binPath = dir + string(os.PathSeparator) + name
		cmd := exec.CommandContext(t.Context(), "go", "build", "-o", binPath, "github.com/nupagachi/agentic-rag/api/cmd/app")
		if out, err := cmd.CombinedOutput(); err != nil {
			errBuild = errors.New("go build failed: " + string(out))
		}
	})
	if errBuild != nil {
		t.Fatalf("build app binary: %v", errBuild)
	}
	return binPath
}

// apiProcess is a running `app api` subprocess.
type apiProcess struct {
	addr    string
	out     *syncBuffer
	secrets []string // known passwords, removed from anything printed on failure
	cmd     *exec.Cmd
	done    chan struct{}
	signal  sync.Once // the stop request is sent at most once
	stopped sync.Once
}

// freeAddr returns a loopback address with a currently free port.
func freeAddr(t *testing.T) string {
	t.Helper()
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find free port: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	return addr
}

// startAPI runs the real binary with exactly the given configuration and waits
// until /healthz answers. Variables the API reads are removed from the
// inherited environment so the developer's shell cannot leak in.
//
// The process is tied to t.Context(): when the test ends it is asked to stop
// the same way stop does (SIGINT; Kill on Windows), and force-killed after
// shutdownTimeout.
func startAPI(t *testing.T, databaseURL, redisURL string) *apiProcess {
	t.Helper()
	bin := appBinary(t)

	p := &apiProcess{
		addr:    freeAddr(t),
		out:     &syncBuffer{},
		secrets: append(passwordsOf(databaseURL), passwordsOf(redisURL)...),
		done:    make(chan struct{}),
	}
	p.cmd = exec.CommandContext(t.Context(), bin, "api")
	p.cmd.Cancel = func() error { p.requestStop(); return nil }
	p.cmd.WaitDelay = shutdownTimeout
	p.cmd.Env = append(filteredEnv("API_ADDR", "DATABASE_URL", "REDIS_URL", "LOG_LEVEL", "CORS_ALLOWED_ORIGINS"),
		"API_ADDR="+p.addr,
		"DATABASE_URL="+databaseURL,
		"REDIS_URL="+redisURL,
		"LOG_LEVEL=info",
	)
	p.cmd.Stdout = p.out
	p.cmd.Stderr = p.out
	if err := p.cmd.Start(); err != nil {
		t.Fatalf("start api: %v", err)
	}
	go func() {
		_ = p.cmd.Wait()
		close(p.done)
	}()
	t.Cleanup(func() { p.stop(t) })

	eventually(t, startupTimeout, "api answers /healthz", func() (bool, string) {
		select {
		case <-p.done:
			t.Fatalf("api exited during startup (exit code %d); output:\n%s",
				p.cmd.ProcessState.ExitCode(), redact(p.out.String(), p.secrets...))
		default:
		}
		status, _, err := p.get("/healthz")
		return err == nil && status == http.StatusOK, "not answering yet"
	})
	return p
}

// requestStop asks the process to stop, once: SIGINT where the OS supports
// sending it to a child, Kill on Windows.
func (p *apiProcess) requestStop() {
	p.signal.Do(func() {
		if runtime.GOOS == "windows" {
			_ = p.cmd.Process.Kill()
			return
		}
		_ = p.cmd.Process.Signal(os.Interrupt)
	})
}

// stop ends the process and returns everything it printed. Safe to call
// twice. Where SIGINT is available (not Windows) it also checks the graceful
// path: the process must exit with code 0 within shutdownTimeout and log
// "shutdown complete".
func (p *apiProcess) stop(t *testing.T) string {
	t.Helper()
	p.stopped.Do(func() {
		select {
		case <-p.done:
			return
		default:
		}
		p.requestStop()
		select {
		case <-p.done:
		case <-time.After(shutdownTimeout):
			_ = p.cmd.Process.Kill()
			<-p.done
			t.Errorf("api did not exit within %s after the stop request", shutdownTimeout)
			return
		}
		if runtime.GOOS == "windows" {
			return
		}
		if code := p.cmd.ProcessState.ExitCode(); code != 0 {
			t.Errorf("api exit code after SIGINT = %d, want 0; output:\n%s", code, redact(p.out.String(), p.secrets...))
		}
		if !strings.Contains(p.out.String(), "shutdown complete") {
			t.Errorf("api output has no \"shutdown complete\" line; output:\n%s", redact(p.out.String(), p.secrets...))
		}
	})
	<-p.done
	return p.out.String()
}

// get issues GET path and returns status and body.
func (p *apiProcess) get(path string) (int, []byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return httpGet(ctx, "http://"+p.addr+path)
}

func httpGet(ctx context.Context, rawURL string) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return 0, nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	return resp.StatusCode, body, err
}

// filteredEnv returns the process environment without the named variables.
func filteredEnv(drop ...string) []string {
	var out []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		skip := false
		for _, d := range drop {
			if strings.EqualFold(name, d) {
				skip = true
				break
			}
		}
		if !skip {
			out = append(out, kv)
		}
	}
	return out
}

// eventually polls cond every 50ms until it returns true or timeout passes.
// The reason returned by the last attempt is reported on failure. It is a
// polling wait, never a fixed sleep.
func eventually(t *testing.T, timeout time.Duration, what string, cond func() (bool, string)) {
	t.Helper()
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	for {
		ok, reason := cond()
		if ok {
			return
		}
		select {
		case <-deadline.C:
			t.Fatalf("timed out after %s waiting for: %s (%s)", timeout, what, reason)
		case <-tick.C:
		}
	}
}

// readyzBody is the exact shape /readyz may return.
type readyzBody struct {
	Status  string   `json:"status"`
	Failing []string `json:"failing"`
}

// decodeReadyz parses a /readyz body strictly: any unknown field fails.
func decodeReadyz(t *testing.T, body []byte) readyzBody {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	var b readyzBody
	if err := dec.Decode(&b); err != nil {
		t.Fatalf("/readyz body is not the documented shape: %v; body: %q", err, body)
	}
	return b
}

// forbiddenInBody are fragments that would reveal a DSN or a connection error
// (SR-8). The documented failing names ("postgres", "redis") are not in it.
var forbiddenInBody = []string{
	sentinel, "://", "refused", "dial", "tcp", "127.0.0.1", "password", "connect", "timeout", "error",
}

// assertBodyHasNoConnectionDetails fails when a 503 body contains anything
// beyond the fixed shape or a fragment of a DSN or connection error (SR-8).
func assertBodyHasNoConnectionDetails(t *testing.T, body []byte) {
	t.Helper()
	lower := strings.ToLower(string(body))
	for _, f := range forbiddenInBody {
		if strings.Contains(lower, strings.ToLower(f)) {
			t.Errorf("response body contains %q, which could reveal a DSN or connection error: %q", f, body)
		}
	}
}

// assertNoSecrets fails when text contains any non-empty secret.
func assertNoSecrets(t *testing.T, what, text string, secrets ...string) {
	t.Helper()
	for _, s := range secrets {
		if s != "" && strings.Contains(text, s) {
			t.Errorf("%s contains a secret value (len %d)", what, len(s))
		}
	}
}
