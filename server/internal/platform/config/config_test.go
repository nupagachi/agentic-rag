package config

import (
	"strings"
	"testing"
	"time"
)

func valid() Config {
	return Config{
		Env:      EnvDevelopment,
		HTTP:     HTTP{ShutdownTimeout: time.Second},
		Log:      Log{Format: "json"},
		Postgres: Postgres{MaxOpenConns: 1},
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr string
	}{
		{name: "valid", mutate: func(*Config) {}},
		{name: "unknown env", mutate: func(c *Config) { c.Env = "prod" }, wantErr: "APP_ENV"},
		{name: "unknown log format", mutate: func(c *Config) { c.Log.Format = "xml" }, wantErr: "LOG_FORMAT"},
		{name: "zero shutdown timeout", mutate: func(c *Config) { c.HTTP.ShutdownTimeout = 0 }, wantErr: "HTTP_SHUTDOWN_TIMEOUT"},
		{name: "negative body limit", mutate: func(c *Config) { c.HTTP.MaxBodyBytes = -1 }, wantErr: "HTTP_MAX_BODY_BYTES"},
		{name: "zero pool", mutate: func(c *Config) { c.Postgres.MaxOpenConns = 0 }, wantErr: "POSTGRES_MAX_OPEN_CONNS"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := valid()
			tt.mutate(&cfg)
			err := cfg.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want mention of %s", err, tt.wantErr)
			}
		})
	}
}

func TestLoadParsesTrustedProxies(t *testing.T) {
	t.Chdir(t.TempDir()) // không đọc .env của dev
	t.Setenv("POSTGRES_DSN", "postgres://x@127.0.0.1/x")
	t.Setenv("HTTP_TRUSTED_PROXIES", "10.0.0.0/8,172.16.0.1")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(cfg.HTTP.TrustedProxies, ","); got != "10.0.0.0/8,172.16.0.1" {
		t.Fatalf("TrustedProxies = %q", got)
	}
}
