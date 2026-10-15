// Package cache tạo Redis client.
package cache

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"

	"github.com/nhucanh/agentic-rag/server/internal/platform/config"
	"github.com/nhucanh/agentic-rag/server/internal/platform/lifecycle"
)

// New tạo client; go-redis kết nối lười nên không lỗi khi Redis chưa chạy.
// Client được đóng khi lifecycle dừng.
func New(cfg *config.Config, log *logrus.Logger, lc *lifecycle.Lifecycle) *redis.Client {
	redis.SetLogger(logAdapter{log.WithField("component", "redis")})
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	lc.OnStop("redis", func(context.Context) error { return rdb.Close() })
	return rdb
}

// logAdapter chuyển log nội bộ của go-redis sang logrus (JSON).
type logAdapter struct{ e *logrus.Entry }

func (a logAdapter) Printf(_ context.Context, format string, v ...any) {
	a.e.Warn(fmt.Sprintf(format, v...))
}
