package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"

	"github.com/redis/go-redis/v9"

	"github.com/nupagachi/agentic-rag/api/internal/platform/config"
	"github.com/nupagachi/agentic-rag/api/internal/platform/db"
	"github.com/nupagachi/agentic-rag/api/internal/platform/health"
	"github.com/nupagachi/agentic-rag/api/internal/platform/logx"
	"github.com/nupagachi/agentic-rag/api/internal/platform/redisx"
	"github.com/nupagachi/agentic-rag/api/internal/platform/server"
)

// runAPI implements "app api". Configuration errors go to stderr as plain
// text (the logger needs a valid LOG_LEVEL, which is part of what failed);
// everything after that is JSON on stdout.
func runAPI(ctx context.Context, args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("api", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { _, _ = fmt.Fprint(stderr, usageText) }
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() > 0 {
		_, _ = fmt.Fprintf(stderr, "app api: unexpected argument\n\n%s", usageText)
		return 2
	}

	cfg, err := config.Load(getenv)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "app api: %v\n", err)
		return 1
	}

	log := logx.New(stdout, cfg.LogLevel, logx.NewRedactor(cfg.DatabaseURL, cfg.RedisURL))
	log.Info("starting", slog.Any("config", cfg))

	// go-redis logs through a process-wide global; route it to the redacting
	// logger here, once, instead of from inside a constructor.
	redis.SetLogger(redisx.NewLogger(log))

	lc := net.ListenConfig{}
	ln, err := lc.Listen(ctx, "tcp", cfg.APIAddr)
	if err != nil {
		log.Error("listen failed", slog.String("addr", cfg.APIAddr), slog.Any("error", err))
		return 1
	}

	if err := serveAPI(ctx, cfg, log, ln); err != nil {
		log.Error("api stopped with error", slog.Any("error", err))
		return 1
	}
	log.Info("shutdown complete")
	return 0
}

// serveAPI wires dependencies, serves on ln until ctx is cancelled, then
// drains HTTP and closes the Postgres pool and Redis client.
func serveAPI(ctx context.Context, cfg config.Config, log *slog.Logger, ln net.Listener) error {
	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		_ = ln.Close()
		return err
	}
	defer pool.Close()

	rdb, err := redisx.Open(cfg.RedisURL)
	if err != nil {
		_ = ln.Close()
		return err
	}
	defer func() {
		if cerr := rdb.Close(); cerr != nil {
			log.Warn("closing redis client", slog.Any("error", cerr))
		}
	}()

	probes := health.NewChecker(log, health.DefaultTimeout,
		health.Check{Name: health.Postgres, Pinger: pool},
		health.Check{Name: health.Redis, Pinger: redisx.NewPinger(rdb)},
	)
	router := server.NewRouter(server.Options{}, probes)
	srv := server.New(cfg.APIAddr, router, log)

	log.Info("listening", slog.String("addr", ln.Addr().String()))
	return server.Serve(ctx, srv, ln, server.ShutdownTimeout)
}
