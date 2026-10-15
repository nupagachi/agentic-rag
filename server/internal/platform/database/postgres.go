// Package database mở kết nối Postgres qua GORM.
package database

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/nhucanh/agentic-rag/server/internal/platform/config"
	"github.com/nhucanh/agentic-rag/server/internal/platform/lifecycle"
)

// New mở pool nhưng không ping: API vẫn khởi động khi Postgres chưa sẵn sàng,
// /readyz sẽ báo trạng thái thật. Pool được đóng khi lifecycle dừng.
func New(cfg *config.Config, log *logrus.Logger, lc *lifecycle.Lifecycle) (*gorm.DB, error) {
	gormLog := gormlogger.New(log, gormlogger.Config{
		SlowThreshold:             500 * time.Millisecond,
		LogLevel:                  gormlogger.Warn,
		IgnoreRecordNotFoundError: true,
	})

	db, err := gorm.Open(postgres.Open(cfg.Postgres.DSN), &gorm.Config{
		Logger:               gormLog,
		DisableAutomaticPing: true,
		TranslateError:       true,
	})
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(cfg.Postgres.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.Postgres.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.Postgres.ConnMaxLifetime)

	lc.OnStop("postgres", func(context.Context) error { return sqlDB.Close() })
	return db, nil
}

func Ping(ctx context.Context, db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}
