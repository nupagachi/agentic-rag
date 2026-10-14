//go:build integration

package integration

import (
	"net/http"
	"reflect"
	"testing"
	"time"
)

// AC1: with the dev stack healthy, the real binary reports ready.
func TestReadyz_ReturnsOKWhenStackIsHealthy(t *testing.T) {
	s := requireStack(t)
	p := startAPI(t, s.databaseURL, s.redisURL)

	status, body, err := p.get("/readyz")
	if err != nil {
		t.Fatalf("GET /readyz: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %q", status, body)
	}
	got := decodeReadyz(t, body)
	if got.Status != "ok" || len(got.Failing) != 0 {
		t.Fatalf(`body = %q, want {"status":"ok"}`, body)
	}

	out := p.stop(t)
	assertNoSecrets(t, "process output", out, append(passwordsOf(s.databaseURL), passwordsOf(s.redisURL)...)...)
}

// AC2 + SR-8: Postgres on a dead port is reported by name only, quickly.
func TestReadyz_PostgresDownReturns503NamingOnlyPostgres(t *testing.T) {
	s := requireStack(t) // real Redis, so postgres is the only failing dependency
	deadDB := "postgres://app:" + sentinel + "@" + deadAddr + "/app?sslmode=disable"
	p := startAPI(t, deadDB, s.redisURL)

	began := time.Now()
	status, body, err := p.get("/readyz")
	elapsed := time.Since(began)
	if err != nil {
		t.Fatalf("GET /readyz: %v", err)
	}

	if status != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body: %q", status, body)
	}
	if got := decodeReadyz(t, body); !reflect.DeepEqual(got.Failing, []string{"postgres"}) {
		t.Errorf("failing = %v, want [postgres]", got.Failing)
	}
	if elapsed > 1500*time.Millisecond {
		t.Errorf("/readyz took %s, want <= 1.5s", elapsed)
	}
	assertBodyHasNoConnectionDetails(t, body)

	out := p.stop(t)
	assertNoSecrets(t, "process output", out, append(passwordsOf(s.redisURL), sentinel)...)
}

// SR-8: both dependencies on dead ports. Needs no stack, so it always runs.
func TestReadyz_AllDependenciesDownReturns503WithNamesOnly(t *testing.T) {
	deadDB := "postgres://app:" + sentinel + "@" + deadAddr + "/app?sslmode=disable"
	deadRedis := "redis://:" + sentinel + "@" + deadAddr + "/0"
	p := startAPI(t, deadDB, deadRedis)

	began := time.Now()
	status, body, err := p.get("/readyz")
	elapsed := time.Since(began)
	if err != nil {
		t.Fatalf("GET /readyz: %v", err)
	}

	if status != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body: %q", status, body)
	}
	got := decodeReadyz(t, body)
	if want := []string{"postgres", "redis"}; !reflect.DeepEqual(got.Failing, want) {
		t.Errorf("failing = %v, want %v", got.Failing, want)
	}
	if elapsed > 1500*time.Millisecond {
		t.Errorf("/readyz took %s, want <= 1.5s", elapsed)
	}
	assertBodyHasNoConnectionDetails(t, body)

	out := p.stop(t)
	assertNoSecrets(t, "process output", out, sentinel)
}

// SR-2: a wrong Redis password makes /readyz fail on redis, and the wrong
// password never appears in the response or the logs.
func TestReadyz_WrongRedisPasswordReturns503NamingOnlyRedis(t *testing.T) {
	s := requireStack(t)
	wrong := withPassword(t, s.redisURL, "wrong-"+sentinel)
	p := startAPI(t, s.databaseURL, wrong)

	status, body, err := p.get("/readyz")
	if err != nil {
		t.Fatalf("GET /readyz: %v", err)
	}

	if status != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body: %q", status, body)
	}
	if got := decodeReadyz(t, body); !reflect.DeepEqual(got.Failing, []string{"redis"}) {
		t.Errorf("failing = %v, want [redis]", got.Failing)
	}
	assertBodyHasNoConnectionDetails(t, body)

	out := p.stop(t)
	assertNoSecrets(t, "process output", out, append(append(passwordsOf(s.redisURL), passwordsOf(s.databaseURL)...), "wrong-"+sentinel)...)
}
