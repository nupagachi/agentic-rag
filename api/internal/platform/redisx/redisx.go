// Package redisx builds the Redis client and the small adapters around it.
package redisx

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/nupagachi/agentic-rag/api/internal/platform/logx"
)

// Timeouts for a single Redis operation.
const (
	DialTimeout  = 5 * time.Second
	ReadTimeout  = 3 * time.Second
	WriteTimeout = 3 * time.Second
)

// ErrInvalidURL is returned when the URL cannot be parsed. It carries no
// detail: parser errors can echo the URL.
var ErrInvalidURL = errors.New("redisx: REDIS_URL is not a valid Redis URL")

// Open builds the client. go-redis connects lazily, so an unreachable server
// does not fail startup; /readyz reports it instead.
//
// go-redis' own logging is a process-wide global; Open does not touch it.
// The composition root decides, with redis.SetLogger(NewLogger(log)).
func Open(url logx.Secret) (*redis.Client, error) {
	opts, err := redis.ParseURL(url.Reveal())
	if err != nil {
		return nil, ErrInvalidURL
	}
	opts.DialTimeout = DialTimeout
	opts.ReadTimeout = ReadTimeout
	opts.WriteTimeout = WriteTimeout
	return redis.NewClient(opts), nil
}

// Pinger adapts a client to the health.Pinger shape: *redis.Client.Ping
// returns a *StatusCmd, not an error.
type Pinger struct {
	client *redis.Client
}

// NewPinger returns a Pinger that probes c.
func NewPinger(c *redis.Client) Pinger { return Pinger{client: c} }

// Ping implements health.Pinger.
func (p Pinger) Ping(ctx context.Context) error {
	return p.client.Ping(ctx).Err()
}

// Logger sends go-redis internal messages (pool and dial failures) to slog so
// they pass through the redacting handler instead of raw stderr. It implements
// go-redis' internal Logging interface.
type Logger struct {
	log *slog.Logger
}

// NewLogger returns a Logger that writes to log. log must be the redacting
// logger.
func NewLogger(log *slog.Logger) Logger { return Logger{log: log} }

// Printf implements go-redis' internal Logging interface.
func (l Logger) Printf(ctx context.Context, format string, v ...any) {
	l.log.Log(ctx, slog.LevelWarn, fmt.Sprintf(format, v...), slog.String("component", "go-redis"))
}
