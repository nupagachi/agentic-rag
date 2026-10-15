// Package app ráp các thành phần bằng dig và chạy vòng đời server.
package app

import (
	"context"

	"github.com/redis/go-redis/v9"
	"go.uber.org/dig"
	"gorm.io/gorm"

	"github.com/nhucanh/agentic-rag/server/internal/apidocs"
	"github.com/nhucanh/agentic-rag/server/internal/health"
	"github.com/nhucanh/agentic-rag/server/internal/platform/cache"
	"github.com/nhucanh/agentic-rag/server/internal/platform/config"
	"github.com/nhucanh/agentic-rag/server/internal/platform/database"
	"github.com/nhucanh/agentic-rag/server/internal/platform/httpx"
	"github.com/nhucanh/agentic-rag/server/internal/platform/lifecycle"
	"github.com/nhucanh/agentic-rag/server/internal/platform/logger"
)

// BuildContainer khai báo mọi provider. Domain mới: provide service/repo,
// còn handler thì provide vào group "routes" dưới dạng httpx.Routes.
// Kho/dịch vụ ngoài mà /readyz cần kiểm tra: provide health.Check vào group "health_checks".
func BuildContainer() (*dig.Container, error) {
	c := dig.New()
	routes := []dig.ProvideOption{dig.As(new(httpx.Routes)), dig.Group("routes")}
	checks := []dig.ProvideOption{dig.Group("health_checks")}

	providers := []struct {
		fn   any
		opts []dig.ProvideOption
	}{
		// platform
		{fn: config.Load},
		{fn: logger.New},
		{fn: lifecycle.New},
		{fn: database.New},
		{fn: cache.New},
		{fn: httpx.NewRouter},
		{fn: httpx.NewServer},

		// readiness
		{fn: postgresCheck, opts: checks},
		{fn: redisCheck, opts: checks},

		// domains
		{fn: health.NewHandler, opts: routes},
		{fn: apidocs.NewHandler, opts: routes},
	}

	for _, p := range providers {
		if err := c.Provide(p.fn, p.opts...); err != nil {
			return nil, err
		}
	}
	return c, nil
}

func postgresCheck(db *gorm.DB) health.Check {
	return health.Check{Name: "postgres", Ping: func(ctx context.Context) error { return database.Ping(ctx, db) }}
}

func redisCheck(rdb *redis.Client) health.Check {
	return health.Check{Name: "redis", Ping: func(ctx context.Context) error { return rdb.Ping(ctx).Err() }}
}
