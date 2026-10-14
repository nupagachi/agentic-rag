// Package config loads and validates the API configuration from environment
// variables. Errors name the offending variable and the reason only; they
// never contain the value, because values may be credentials (SR-6).
package config

import (
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/nupagachi/agentic-rag/api/internal/platform/logx"
)

// Environment variable names.
const (
	EnvAPIAddr     = "API_ADDR"
	EnvDatabaseURL = "DATABASE_URL"
	EnvRedisURL    = "REDIS_URL"
	EnvLogLevel    = "LOG_LEVEL"
	EnvCORSOrigins = "CORS_ALLOWED_ORIGINS"
)

// DefaultAPIAddr is loopback only; exposing the API is an explicit decision (SR-4).
const DefaultAPIAddr = "127.0.0.1:8080"

// Config is the validated API configuration.
type Config struct {
	APIAddr            string
	DatabaseURL        logx.Secret
	RedisURL           logx.Secret
	LogLevel           slog.Level
	CORSAllowedOrigins []string
}

// Problem is one invalid or missing variable.
type Problem struct {
	Var    string
	Reason string
}

// Error aggregates every problem found so the operator can fix them in one go.
type Error struct {
	Problems []Problem
}

// Error lists "VAR: reason" pairs. It never includes values.
func (e *Error) Error() string {
	parts := make([]string, len(e.Problems))
	for i, p := range e.Problems {
		parts[i] = p.Var + ": " + p.Reason
	}
	return "invalid configuration: " + strings.Join(parts, "; ")
}

// Load reads and validates the configuration. getenv is os.Getenv in
// production and a map lookup in tests.
func Load(getenv func(string) string) (Config, error) {
	var (
		cfg Config
		ps  []Problem
		bad = func(name, reason string) { ps = append(ps, Problem{Var: name, Reason: reason}) }
	)

	cfg.APIAddr = strings.TrimSpace(getenv(EnvAPIAddr))
	if cfg.APIAddr == "" {
		cfg.APIAddr = DefaultAPIAddr
	}
	if reason := validateAddr(cfg.APIAddr); reason != "" {
		bad(EnvAPIAddr, reason)
	}

	cfg.DatabaseURL = logx.NewSecret(strings.TrimSpace(getenv(EnvDatabaseURL)))
	if reason := validateURL(cfg.DatabaseURL.Reveal(), "postgres", "postgresql"); reason != "" {
		bad(EnvDatabaseURL, reason)
	}

	cfg.RedisURL = logx.NewSecret(strings.TrimSpace(getenv(EnvRedisURL)))
	if reason := validateURL(cfg.RedisURL.Reveal(), "redis", "rediss"); reason != "" {
		bad(EnvRedisURL, reason)
	}

	cfg.LogLevel = slog.LevelInfo
	if raw := strings.TrimSpace(getenv(EnvLogLevel)); raw != "" {
		lvl, err := logx.ParseLevel(raw)
		if err != nil {
			bad(EnvLogLevel, "must be one of debug, info, warn, error")
		} else {
			cfg.LogLevel = lvl
		}
	}

	origins, reason := parseOrigins(getenv(EnvCORSOrigins))
	if reason != "" {
		bad(EnvCORSOrigins, reason)
	}
	cfg.CORSAllowedOrigins = origins

	if len(ps) > 0 {
		return Config{}, &Error{Problems: ps}
	}
	return cfg, nil
}

// LogValue renders the config for the startup log with credentials masked.
func (c Config) LogValue() slog.Value {
	origins := c.CORSAllowedOrigins
	if origins == nil {
		origins = []string{}
	}
	return slog.GroupValue(
		slog.String("api_addr", c.APIAddr),
		slog.String("database_url", logx.RedactURL(c.DatabaseURL.Reveal())),
		slog.String("redis_url", logx.RedactURL(c.RedisURL.Reveal())),
		slog.String("log_level", c.LogLevel.String()),
		slog.Any("cors_allowed_origins", origins),
	)
}

// validateAddr requires host:port with a non-empty host, so that a bare
// ":8080" (all interfaces) can never be configured by accident.
func validateAddr(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "must be host:port"
	}
	if host == "" {
		return "must include an explicit host (for example 127.0.0.1:8080)"
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 0 || n > 65535 {
		return "port must be a number between 0 and 65535"
	}
	return ""
}

func validateURL(raw string, schemes ...string) string {
	if raw == "" {
		return "required but not set"
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "is not a valid URL"
	}
	// A fragment or an opaque part almost always means a password with an
	// unescaped "#" (or similar), which would silently change the host.
	if u.Fragment != "" || u.Opaque != "" {
		return "must not contain a fragment or opaque part; special characters in password must be percent-encoded"
	}
	for _, s := range schemes {
		if u.Scheme == s {
			if u.Hostname() == "" {
				return "must include a host"
			}
			return ""
		}
	}
	return fmt.Sprintf("scheme must be one of %s", strings.Join(schemes, ", "))
}

// parseOrigins splits a comma separated list of exact origins. Wildcards are
// rejected (SR-12): credentials plus a wildcard origin is a classic CORS hole.
func parseOrigins(raw string) ([]string, string) {
	var out []string
	for _, o := range strings.Split(raw, ",") {
		o = strings.TrimSpace(o)
		if o == "" {
			continue
		}
		if strings.Contains(o, "*") {
			return nil, "wildcard origins are not allowed; list exact origins"
		}
		u, err := url.Parse(o)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" ||
			(u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
			return nil, "each origin must look like scheme://host[:port]"
		}
		out = append(out, u.Scheme+"://"+u.Host)
	}
	return out, ""
}
