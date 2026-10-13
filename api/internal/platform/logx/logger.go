package logx

import (
	"errors"
	"io"
	"log/slog"
	"strings"
)

// ParseLevel converts debug|info|warn|error (case-insensitive) to a slog level.
func ParseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, errors.New("unknown log level")
	}
}

// New returns a JSON slog logger writing to w at the given level. Every string
// and error attribute (and the message) passes through red, so driver errors
// that embed a DSN cannot leak the password.
func New(w io.Writer, level slog.Level, red *Redactor) *slog.Logger {
	h := slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			switch a.Value.Kind() {
			case slog.KindString:
				a.Value = slog.StringValue(red.Redact(a.Value.String()))
			case slog.KindAny:
				if err, ok := a.Value.Any().(error); ok {
					a.Value = slog.StringValue(red.Redact(err.Error()))
				}
			default:
			}
			return a
		},
	})
	return slog.New(h)
}
