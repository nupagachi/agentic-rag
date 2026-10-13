// Package redisx builds the Redis client.
package redisx

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"

	"agentic-rag/api/internal/platform/logx"
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

// Client wraps *redis.Client and adapts Ping to the health.Pinger shape.
type Client struct {
	*redis.Client
}

// slogAdapter sends go-redis internal messages (pool and dial failures) to
// slog so they pass through the redacting handler instead of raw stderr.
type slogAdapter struct {
	log *slog.Logger
}

// Printf implements go-redis' internal Logging interface.
func (a slogAdapter) Printf(ctx context.Context, format string, v ...any) {
	a.log.Log(ctx, slog.LevelWarn, fmt.Sprintf(format, v...), slog.String("component", "go-redis"))
}

// Open builds the client. go-redis connects lazily, so an unreachable server
// does not fail startup; /readyz reports it instead. log must be the
// redacting logger: go-redis' own logging is global and is redirected to it.
func Open(url logx.Secret, log *slog.Logger) (*Client, error) {
	opts, err := redis.ParseURL(url.Reveal())
	if err != nil {
		return nil, ErrInvalidURL
	}
	if log != nil {
		redis.SetLogger(slogAdapter{log: log})
	}
	opts.DialTimeout = DialTimeout
	opts.ReadTimeout = ReadTimeout
	opts.WriteTimeout = WriteTimeout
	return &Client{Client: redis.NewClient(opts)}, nil
}

// Ping implements health.Pinger.
func (c *Client) Ping(ctx context.Context) error {
	return c.Client.Ping(ctx).Err()
}
