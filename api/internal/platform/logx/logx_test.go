package logx_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/nupagachi/agentic-rag/api/internal/platform/logx"
)

const sentinel = "S3NTINEL"

func TestSecret_NeverPrintsValue(t *testing.T) {
	t.Parallel()

	type holder struct {
		DSN logx.Secret
	}
	s := logx.NewSecret(sentinel)
	h := holder{DSN: s}

	cases := map[string]string{
		"%v":         fmt.Sprintf("%v", s),
		"%+v":        fmt.Sprintf("%+v", s),
		"%#v":        fmt.Sprintf("%#v", s),
		"Sprint":     fmt.Sprint(s),
		"%q":         fmt.Sprintf("%q", s),
		"struct %v":  fmt.Sprintf("%v", h),
		"struct %+v": fmt.Sprintf("%+v", h),
		"struct %#v": fmt.Sprintf("%#v", h),
		"ptr %+v":    fmt.Sprintf("%+v", &h),
	}
	for name, got := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if strings.Contains(got, sentinel) {
				t.Fatalf("output leaks secret: %q", got)
			}
			if !strings.Contains(got, logx.Mask) {
				t.Fatalf("output %q does not contain mask", got)
			}
		})
	}

	if s.Reveal() != sentinel {
		t.Fatalf("Reveal() = %q, want raw value", s.Reveal())
	}
}

func TestSecret_JSONAndSlog(t *testing.T) {
	t.Parallel()

	s := logx.NewSecret(sentinel)

	b, err := json.Marshal(map[string]any{"s": s})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(b), sentinel) {
		t.Fatalf("json leaks secret: %s", b)
	}

	var buf bytes.Buffer
	log := logx.New(&buf, slog.LevelInfo, nil)
	log.Info("check", slog.Any("dsn", s), slog.Group("g", slog.Any("inner", s)))
	if strings.Contains(buf.String(), sentinel) {
		t.Fatalf("slog JSON leaks secret: %s", buf.String())
	}
	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("log line is not JSON: %v", err)
	}
	if rec["dsn"] != logx.Mask {
		t.Fatalf("dsn = %v, want mask", rec["dsn"])
	}
}

func TestRedactURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"postgres password", "postgres://app:S3NTINEL@127.0.0.1:5432/app?sslmode=disable", "postgres://app:***@127.0.0.1:5432/app?sslmode=disable"},
		{"redis empty user", "redis://:S3NTINEL@127.0.0.1:6379/0", "redis://:***@127.0.0.1:6379/0"},
		{"no password", "postgres://app@127.0.0.1:5432/app", "postgres://app@127.0.0.1:5432/app"},
		{"password query", "postgres://app@127.0.0.1/app?password=S3NTINEL&sslmode=disable", "postgres://app@127.0.0.1/app?password=***&sslmode=disable"},
		{"not a url", "S3NTINEL", logx.Mask},
		{"unparseable", "postgres://app:S3NTINEL@host:notaport/db", logx.Mask},
		{"empty", "", logx.Mask},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := logx.RedactURL(tt.in)
			if got != tt.want {
				t.Fatalf("RedactURL(%q) = %q, want %q", tt.in, got, tt.want)
			}
			if strings.Contains(got, sentinel) {
				t.Fatalf("leaks sentinel: %q", got)
			}
		})
	}
}

func TestRedactor_ScrubsDriverErrors(t *testing.T) {
	t.Parallel()

	dsn := logx.NewSecret("postgres://app:S3NTINEL@127.0.0.1:5432/app")
	red := logx.NewRedactor(dsn, logx.NewSecret(""))

	tests := []struct {
		name string
		in   string
	}{
		{"full dsn", "failed to connect to postgres://app:S3NTINEL@127.0.0.1:5432/app: refused"},
		{"password only", "FATAL: password authentication failed (S3NTINEL)"},
		{"no secret", "connection refused"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := red.Redact(tt.in)
			if strings.Contains(got, sentinel) {
				t.Fatalf("Redact leaks secret: %q", got)
			}
		})
	}

	if got := (*logx.Redactor)(nil).Redact("x"); got != "x" {
		t.Fatalf("nil redactor must be a no-op, got %q", got)
	}
}

func TestNew_RedactsErrorsAndStrings(t *testing.T) {
	t.Parallel()

	red := logx.NewRedactor(logx.NewSecret("redis://:S3NTINEL@127.0.0.1:6379/0"))
	var buf bytes.Buffer
	log := logx.New(&buf, slog.LevelInfo, red)

	err := fmt.Errorf("dial: %w", errors.New("auth failed for S3NTINEL"))
	log.Error("connect S3NTINEL failed", slog.Any("error", err), slog.String("detail", "pw=S3NTINEL"))

	out := buf.String()
	if strings.Contains(out, sentinel) {
		t.Fatalf("log output leaks secret: %s", out)
	}
	if !strings.Contains(out, "dial: auth failed for ***") {
		t.Fatalf("expected redacted error text, got %s", out)
	}
}

func TestNew_RespectsLevel(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	log := logx.New(&buf, slog.LevelWarn, nil)
	log.Info("hidden")
	log.Warn("shown")
	if strings.Contains(buf.String(), "hidden") || !strings.Contains(buf.String(), "shown") {
		t.Fatalf("level filtering broken: %s", buf.String())
	}
}

func TestParseLevel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in      string
		want    slog.Level
		wantErr bool
	}{
		{"debug", slog.LevelDebug, false},
		{"INFO", slog.LevelInfo, false},
		{" warn ", slog.LevelWarn, false},
		{"warning", slog.LevelWarn, false},
		{"error", slog.LevelError, false},
		{"trace", 0, true},
		{"", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()
			got, err := logx.ParseLevel(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Fatalf("level = %v, want %v", got, tt.want)
			}
		})
	}
}
