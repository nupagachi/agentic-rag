// Package config đọc cấu hình từ biến môi trường (và file .env khi dev).
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

const (
	EnvDevelopment = "development"
	EnvProduction  = "production"
)

type Config struct {
	Env      string `env:"APP_ENV" envDefault:"development"`
	HTTP     HTTP
	Log      Log
	Postgres Postgres
	Redis    Redis
	APIDocs  APIDocs
}

type HTTP struct {
	Addr            string        `env:"HTTP_ADDR" envDefault:":8080"`
	ReadTimeout     time.Duration `env:"HTTP_READ_TIMEOUT" envDefault:"15s"`
	WriteTimeout    time.Duration `env:"HTTP_WRITE_TIMEOUT" envDefault:"0s"` // 0: không giới hạn, cần cho SSE
	IdleTimeout     time.Duration `env:"HTTP_IDLE_TIMEOUT" envDefault:"60s"`
	ShutdownTimeout time.Duration `env:"HTTP_SHUTDOWN_TIMEOUT" envDefault:"20s"`
	// DrainDelay: chờ sau khi /readyz chuyển 503, trước khi đóng listener,
	// để LB kịp ngừng gửi request. Production nên ≥ chu kỳ health check (vd 5s).
	DrainDelay time.Duration `env:"HTTP_DRAIN_DELAY" envDefault:"0s"`
	// MaxBodyBytes giới hạn body mỗi request (khớp client_max_body_size của Nginx).
	// File lớn đi thẳng MinIO qua presigned URL nên không cần lớn hơn. 0: không giới hạn.
	MaxBodyBytes int64 `env:"HTTP_MAX_BODY_BYTES" envDefault:"2097152"`
	// TrustedProxies: IP/CIDR của Nginx; chỉ khi đó X-Forwarded-For mới được tin để lấy IP client.
	TrustedProxies []string `env:"HTTP_TRUSTED_PROXIES" envSeparator:","`
}

// APIDocs: Swagger UI tại /api/docs và spec tại /api/openapi.yaml.
type APIDocs struct {
	Enabled bool `env:"API_DOCS_ENABLED" envDefault:"true"`
}

type Log struct {
	Level  string `env:"LOG_LEVEL" envDefault:"info"`
	Format string `env:"LOG_FORMAT" envDefault:"json"` // json | text
}

type Postgres struct {
	DSN             string        `env:"POSTGRES_DSN,required"`
	MaxOpenConns    int           `env:"POSTGRES_MAX_OPEN_CONNS" envDefault:"25"`
	MaxIdleConns    int           `env:"POSTGRES_MAX_IDLE_CONNS" envDefault:"5"`
	ConnMaxLifetime time.Duration `env:"POSTGRES_CONN_MAX_LIFETIME" envDefault:"30m"`
}

type Redis struct {
	Addr     string `env:"REDIS_ADDR" envDefault:"127.0.0.1:6379"`
	Password string `env:"REDIS_PASSWORD"`
	DB       int    `env:"REDIS_DB" envDefault:"0"`
}

func (c *Config) IsProduction() bool { return c.Env == EnvProduction }

// Load đọc .env (nếu có, không ghi đè biến đã đặt), parse env vào Config rồi
// kiểm tra giá trị để cấu hình sai báo lỗi ngay lúc khởi động.
func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("load .env: %w", err)
	}

	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	return &cfg, nil
}

// Validate kiểm tra các giá trị env không tự bắt được (enum, khoảng giá trị).
func (c *Config) Validate() error {
	var errs []error
	if c.Env != EnvDevelopment && c.Env != EnvProduction {
		errs = append(errs, fmt.Errorf("APP_ENV=%q, want %s | %s", c.Env, EnvDevelopment, EnvProduction))
	}
	if c.Log.Format != "json" && c.Log.Format != "text" {
		errs = append(errs, fmt.Errorf("LOG_FORMAT=%q, want json | text", c.Log.Format))
	}
	if c.HTTP.ShutdownTimeout <= 0 {
		errs = append(errs, fmt.Errorf("HTTP_SHUTDOWN_TIMEOUT=%s, must be > 0", c.HTTP.ShutdownTimeout))
	}
	if c.HTTP.DrainDelay < 0 {
		errs = append(errs, fmt.Errorf("HTTP_DRAIN_DELAY=%s, must be >= 0", c.HTTP.DrainDelay))
	}
	if c.HTTP.MaxBodyBytes < 0 {
		errs = append(errs, fmt.Errorf("HTTP_MAX_BODY_BYTES=%d, must be >= 0", c.HTTP.MaxBodyBytes))
	}
	if c.Postgres.MaxOpenConns <= 0 {
		errs = append(errs, fmt.Errorf("POSTGRES_MAX_OPEN_CONNS=%d, must be > 0", c.Postgres.MaxOpenConns))
	}
	return errors.Join(errs...)
}
