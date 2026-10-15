// Package logger tạo logrus logger dùng chung và mang logger theo context.Context
// để service/repo ghi log có request_id mà không phụ thuộc Gin.
package logger

import (
	"context"
	"os"

	"github.com/sirupsen/logrus"

	"github.com/nhucanh/agentic-rag/server/internal/platform/config"
)

func New(cfg *config.Config) (*logrus.Logger, error) {
	level, err := logrus.ParseLevel(cfg.Log.Level)
	if err != nil {
		return nil, err
	}

	var formatter logrus.Formatter = &logrus.JSONFormatter{
		TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
		FieldMap:        logrus.FieldMap{logrus.FieldKeyMsg: "message"},
	}
	if cfg.Log.Format == "text" {
		formatter = &logrus.TextFormatter{FullTimestamp: true}
	}

	log := logrus.New()
	log.SetOutput(os.Stdout)
	log.SetLevel(level)
	log.SetFormatter(formatter)

	// Logger mặc định (FromContext khi ctx không mang logger) cùng định dạng.
	std := logrus.StandardLogger()
	std.SetOutput(os.Stdout)
	std.SetLevel(level)
	std.SetFormatter(formatter)
	return log, nil
}

type ctxKey struct{}

// WithContext gắn entry (thường đã có request_id) vào ctx.
func WithContext(ctx context.Context, e *logrus.Entry) context.Context {
	return context.WithValue(ctx, ctxKey{}, e)
}

// FromContext trả logger gắn trong ctx; không có thì trả logger mặc định.
// Dùng trong service/repo: logger.FromContext(ctx).WithField(...).Info(...)
func FromContext(ctx context.Context) *logrus.Entry {
	if e, ok := ctx.Value(ctxKey{}).(*logrus.Entry); ok {
		return e
	}
	return logrus.NewEntry(logrus.StandardLogger())
}
