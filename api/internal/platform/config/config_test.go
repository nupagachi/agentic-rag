package config_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"testing"

	"github.com/nupagachi/agentic-rag/api/internal/platform/config"
	"github.com/nupagachi/agentic-rag/api/internal/platform/logx"
)

const sentinel = "S3NTINEL"

func validEnv() map[string]string {
	return map[string]string{
		"API_ADDR":             "127.0.0.1:8080",
		"DATABASE_URL":         "postgres://app:" + sentinel + "@127.0.0.1:5432/app?sslmode=disable",
		"REDIS_URL":            "redis://:" + sentinel + "@127.0.0.1:6379/0",
		"LOG_LEVEL":            "info",
		"CORS_ALLOWED_ORIGINS": "http://localhost:3000,http://127.0.0.1:3000",
	}
}

func lookup(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func with(mut func(map[string]string)) map[string]string {
	m := validEnv()
	mut(m)
	return m
}

func problemVars(t *testing.T, err error) []string {
	t.Helper()
	var ce *config.Error
	if !errors.As(err, &ce) {
		t.Fatalf("error %v is not *config.Error", err)
	}
	vars := make([]string, len(ce.Problems))
	for i, p := range ce.Problems {
		vars[i] = p.Var
	}
	return vars
}

func TestConfig_Valid(t *testing.T) {
	t.Parallel()

	cfg, err := config.Load(lookup(validEnv()))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.APIAddr != "127.0.0.1:8080" {
		t.Errorf("APIAddr = %q", cfg.APIAddr)
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel = %v", cfg.LogLevel)
	}
	want := []string{"http://localhost:3000", "http://127.0.0.1:3000"}
	if !reflect.DeepEqual(cfg.CORSAllowedOrigins, want) {
		t.Errorf("CORSAllowedOrigins = %v, want %v", cfg.CORSAllowedOrigins, want)
	}
	if !strings.HasPrefix(cfg.DatabaseURL.Reveal(), "postgres://") {
		t.Errorf("DatabaseURL not carried through")
	}
}

func TestConfig_DefaultAddrLoopback(t *testing.T) {
	t.Parallel()

	cfg, err := config.Load(lookup(with(func(m map[string]string) { delete(m, "API_ADDR") })))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.APIAddr != "127.0.0.1:8080" {
		t.Fatalf("default APIAddr = %q, want 127.0.0.1:8080", cfg.APIAddr)
	}
}

func TestConfig_AddrWithoutHost_Rejected(t *testing.T) {
	t.Parallel()

	for _, addr := range []string{":8080", ":0", "8080", "localhost", "127.0.0.1:http", "127.0.0.1:99999"} {
		t.Run(addr, func(t *testing.T) {
			t.Parallel()
			_, err := config.Load(lookup(with(func(m map[string]string) { m["API_ADDR"] = addr })))
			if err == nil {
				t.Fatalf("API_ADDR=%q accepted, want error", addr)
			}
			if got := problemVars(t, err); !reflect.DeepEqual(got, []string{"API_ADDR"}) {
				t.Fatalf("problem vars = %v, want [API_ADDR]", got)
			}
		})
	}
}

func TestConfig_CORSWildcard_Rejected(t *testing.T) {
	t.Parallel()

	for _, v := range []string{"*", "http://a.example,*", "https://*.example.com"} {
		t.Run(v, func(t *testing.T) {
			t.Parallel()
			_, err := config.Load(lookup(with(func(m map[string]string) { m["CORS_ALLOWED_ORIGINS"] = v })))
			if err == nil {
				t.Fatalf("CORS_ALLOWED_ORIGINS=%q accepted, want error", v)
			}
			if got := problemVars(t, err); !reflect.DeepEqual(got, []string{"CORS_ALLOWED_ORIGINS"}) {
				t.Fatalf("problem vars = %v", got)
			}
		})
	}
}

func TestConfig_CORSOrigins(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		in      string
		want    []string
		wantErr bool
	}{
		{"empty is allowed", "", nil, false},
		{"trims and skips blanks", " http://a.example , ,https://b.example:8443/ ", []string{"http://a.example", "https://b.example:8443"}, false},
		{"path rejected", "http://a.example/app", nil, true},
		{"no scheme rejected", "a.example", nil, true},
		{"ftp rejected", "ftp://a.example", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg, err := config.Load(lookup(with(func(m map[string]string) { m["CORS_ALLOWED_ORIGINS"] = tt.in })))
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && !reflect.DeepEqual(cfg.CORSAllowedOrigins, tt.want) {
				t.Fatalf("origins = %v, want %v", cfg.CORSAllowedOrigins, tt.want)
			}
		})
	}
}

func TestConfig_MissingOrInvalid_NamesVarNotValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mut  func(map[string]string)
		want []string
	}{
		{"database missing", func(m map[string]string) { delete(m, "DATABASE_URL") }, []string{"DATABASE_URL"}},
		{"redis missing", func(m map[string]string) { delete(m, "REDIS_URL") }, []string{"REDIS_URL"}},
		{"database wrong scheme", func(m map[string]string) { m["DATABASE_URL"] = "mysql://u:" + sentinel + "@h/db" }, []string{"DATABASE_URL"}},
		{"database unparseable", func(m map[string]string) { m["DATABASE_URL"] = "postgres://u:" + sentinel + "@h:notaport/db" }, []string{"DATABASE_URL"}},
		{"database no host", func(m map[string]string) { m["DATABASE_URL"] = "postgres://u:" + sentinel + "@/db" }, []string{"DATABASE_URL"}},
		{"redis wrong scheme", func(m map[string]string) { m["REDIS_URL"] = "http://:" + sentinel + "@h" }, []string{"REDIS_URL"}},
		{"redis not a url", func(m map[string]string) { m["REDIS_URL"] = sentinel }, []string{"REDIS_URL"}},
		{"bad level", func(m map[string]string) { m["LOG_LEVEL"] = "loud" + sentinel }, []string{"LOG_LEVEL"}},
		{"all bad", func(m map[string]string) {
			m["API_ADDR"] = ":1"
			m["DATABASE_URL"] = sentinel
			m["REDIS_URL"] = ""
		}, []string{"API_ADDR", "DATABASE_URL", "REDIS_URL"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := config.Load(lookup(with(tt.mut)))
			if err == nil {
				t.Fatal("want error")
			}
			if got := problemVars(t, err); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("problem vars = %v, want %v", got, tt.want)
			}
			for _, name := range tt.want {
				if !strings.Contains(err.Error(), name) {
					t.Errorf("message %q does not name %s", err.Error(), name)
				}
			}
			for _, format := range []string{"%v", "%+v", "%#v", "%s"} {
				if strings.Contains(fmt.Sprintf(format, err), sentinel) {
					t.Errorf("error formatted with %s leaks the value: %v", format, err)
				}
			}
		})
	}
}

func TestConfig_SecretsMaskedEverywhere(t *testing.T) {
	t.Parallel()

	cfg, err := config.Load(lookup(validEnv()))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	for _, format := range []string{"%v", "%+v", "%#v"} {
		if out := fmt.Sprintf(format, cfg); strings.Contains(out, sentinel) {
			t.Errorf("fmt %s leaks secret: %s", format, out)
		}
		if out := fmt.Sprintf(format, &cfg); strings.Contains(out, sentinel) {
			t.Errorf("fmt %s of pointer leaks secret: %s", format, out)
		}
	}

	b, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("json: %v", err)
	}
	if strings.Contains(string(b), sentinel) {
		t.Errorf("json leaks secret: %s", b)
	}

	var buf bytes.Buffer
	log := logx.New(&buf, slog.LevelInfo, nil)
	log.Info("starting", slog.Any("config", cfg))
	out := buf.String()
	if strings.Contains(out, sentinel) {
		t.Fatalf("startup log leaks secret: %s", out)
	}
	if !strings.Contains(out, "postgres://app:***@127.0.0.1:5432/app?sslmode=disable") {
		t.Errorf("startup log should show the DSN with masked password, got %s", out)
	}
	if !strings.Contains(out, "redis://:***@127.0.0.1:6379/0") {
		t.Errorf("startup log should show the redis URL with masked password, got %s", out)
	}
}
