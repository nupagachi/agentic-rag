// Package health implements the liveness (/healthz) and readiness (/readyz)
// probes. Probe responses are fixed-shape and never carry error details (SR-8).
package health

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"
)

// DefaultTimeout bounds each readiness probe.
const DefaultTimeout = time.Second

// Dependency is a name from a closed set. Only these names can appear in the
// body of a 503 response.
type Dependency string

// The dependencies the API reports on.
const (
	Postgres Dependency = "postgres"
	Redis    Dependency = "redis"
)

// Pinger is anything that can tell whether it is reachable.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Check pairs a dependency name with the Pinger that probes it.
type Check struct {
	Name   Dependency
	Pinger Pinger
}

// Checker runs readiness checks in parallel.
type Checker struct {
	checks  []Check
	timeout time.Duration
	log     *slog.Logger
}

// NewChecker builds a Checker. A non-positive timeout means DefaultTimeout.
// A nil logger discards failure details.
func NewChecker(log *slog.Logger, timeout time.Duration, checks ...Check) *Checker {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Checker{checks: checks, timeout: timeout, log: log}
}

// Failing pings every dependency concurrently and returns the names of those
// that returned an error or did not answer within the timeout, in the order
// the checks were registered. A Pinger that ignores its context cannot hold
// the caller beyond the timeout.
func (c *Checker) Failing(ctx context.Context) []Dependency {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	type result struct {
		idx int
		err error
	}
	results := make(chan result, len(c.checks)) // buffered: late pingers never block
	for i, chk := range c.checks {
		go func() {
			results <- result{idx: i, err: chk.Pinger.Ping(ctx)}
		}()
	}

	answered := make([]bool, len(c.checks))
	failed := make([]bool, len(c.checks))
	for pending := len(c.checks); pending > 0; pending-- {
		select {
		case r := <-results:
			answered[r.idx] = true
			if r.err != nil {
				failed[r.idx] = true
				name := slog.String("dependency", string(c.checks[r.idx].Name))
				if errors.Is(r.err, context.DeadlineExceeded) {
					// A Pinger that honors ctx reports the timeout as its error.
					c.log.Warn("readiness check timed out", name)
					continue
				}
				// The logger redacts known secrets; the response body never sees err.
				c.log.Warn("readiness check failed", name, slog.Any("error", r.err))
			}
		case <-ctx.Done():
			pending = 0 // stop waiting; unanswered checks count as failed
		}
	}

	var out []Dependency
	for i, chk := range c.checks {
		if !answered[i] {
			// A Pinger that ignores ctx never answered before the deadline.
			c.log.Warn("readiness check timed out", slog.String("dependency", string(chk.Name)))
		}
		if failed[i] || !answered[i] {
			out = append(out, chk.Name)
		}
	}
	return out
}

type body struct {
	Status  string       `json:"status"`
	Failing []Dependency `json:"failing,omitempty"`
}

// Healthz is the liveness probe: the process is up. It checks no dependency.
func (c *Checker) Healthz(w http.ResponseWriter, _ *http.Request) {
	write(w, http.StatusOK, body{Status: "ok"})
}

// Readyz is the readiness probe: 200 when every dependency answers in time,
// otherwise 503 listing only the failing dependency names.
func (c *Checker) Readyz(w http.ResponseWriter, r *http.Request) {
	if failing := c.Failing(r.Context()); len(failing) > 0 {
		write(w, http.StatusServiceUnavailable, body{Status: "unavailable", Failing: failing})
		return
	}
	write(w, http.StatusOK, body{Status: "ok"})
}

func write(w http.ResponseWriter, status int, b body) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	// The client may have gone away; there is nobody to report the error to.
	_ = json.NewEncoder(w).Encode(b)
}
