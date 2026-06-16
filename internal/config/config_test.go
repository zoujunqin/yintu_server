package config

import (
	"os"
	"testing"
)

func TestLoad_DefaultAppEnv(t *testing.T) {
	t.Setenv("APP_ENV", "")
	t.Setenv("POSTGRES_HOST", "")

	cfg := Load()

	if cfg.App.Env != "development" {
		t.Fatalf("expected default APP_ENV=development, got %q", cfg.App.Env)
	}
	if cfg.Postgres.Enabled() {
		t.Fatalf("expected Postgres disabled when host/DSN are empty")
	}
}

func TestLoad_PostgresFromFields(t *testing.T) {
	t.Setenv("POSTGRES_HOST", "127.0.0.1")
	t.Setenv("POSTGRES_PORT", "5433")
	t.Setenv("POSTGRES_USER", "u")
	t.Setenv("POSTGRES_PASSWORD", "p")
	t.Setenv("POSTGRES_DB", "d")
	t.Setenv("POSTGRES_SSLMODE", "disable")
	t.Setenv("POSTGRES_TIMEZONE", "UTC")

	cfg := Load()

	if !cfg.Postgres.Enabled() {
		t.Fatalf("expected Postgres enabled")
	}
	if cfg.Postgres.Port != 5433 {
		t.Fatalf("expected port 5433, got %d", cfg.Postgres.Port)
	}
	want := "host=127.0.0.1 port=5433 user=u password=p dbname=d sslmode=disable TimeZone=UTC"
	if cfg.Postgres.DSN != want {
		t.Fatalf("DSN mismatch:\n want=%q\n got =%q", want, cfg.Postgres.DSN)
	}
}

func TestLoad_PostgresFromDSN(t *testing.T) {
	t.Setenv("POSTGRES_HOST", "")
	t.Setenv("DATABASE_URL", "postgres://u:p@h:5432/d?sslmode=disable")

	cfg := Load()

	if !cfg.Postgres.Enabled() {
		t.Fatalf("expected Postgres enabled via DATABASE_URL")
	}
	if cfg.Postgres.DSN != "postgres://u:p@h:5432/d?sslmode=disable" {
		t.Fatalf("unexpected DSN: %q", cfg.Postgres.DSN)
	}
}

// TestLoad_EnvFileOverrideChain 模拟"低 → 高"覆盖链：
//
//	.env             POSTGRES_PASSWORD=from_env
//	.env.development POSTGRES_PASSWORD=from_dev
//	.env.local       POSTGRES_PASSWORD=from_local
//
// 期望最终值为 from_local，即 .env.local 覆盖 .env.development 覆盖 .env。
func TestLoad_EnvFileOverrideChain(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	// POSTGRES_HOST 必须非空，否则 loadPostgresConfig 会直接返回空 config。
	t.Setenv("POSTGRES_HOST", "127.0.0.1")
	t.Setenv("POSTGRES_PORT", "")
	writeEnvFile(t, "POSTGRES_PASSWORD", ".env", "POSTGRES_PASSWORD=from_env\n")
	writeEnvFile(t, "POSTGRES_PASSWORD", ".env.development", "POSTGRES_PASSWORD=from_dev\n")
	writeEnvFile(t, "POSTGRES_PASSWORD", ".env.local", "POSTGRES_PASSWORD=from_local\n")

	cfg := Load()

	if !cfg.Postgres.Enabled() {
		t.Fatalf("expected Postgres enabled so Password is populated")
	}
	if cfg.Postgres.Password != "from_local" {
		t.Fatalf("expected .env.local to win, got %q", cfg.Postgres.Password)
	}
}

// TestLoad_ProcessEnvWins 进程环境变量必须永远胜出，覆盖任何 .env 文件。
func TestLoad_ProcessEnvWins(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("POSTGRES_HOST", "127.0.0.1")
	t.Setenv("POSTGRES_PORT", "")
	t.Setenv("POSTGRES_PASSWORD", "from_process")
	writeEnvFile(t, "POSTGRES_PASSWORD", ".env", "POSTGRES_PASSWORD=from_env\n")
	writeEnvFile(t, "POSTGRES_PASSWORD", ".env.development", "POSTGRES_PASSWORD=from_dev\n")
	writeEnvFile(t, "POSTGRES_PASSWORD", ".env.local", "POSTGRES_PASSWORD=from_local\n")

	cfg := Load()

	if cfg.Postgres.Password != "from_process" {
		t.Fatalf("expected process env to win, got %q", cfg.Postgres.Password)
	}
}

// TestLoad_DevLocalOverridesLocal 验证 .env.development.local 优先于 .env.local。
func TestLoad_DevLocalOverridesLocal(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("POSTGRES_HOST", "127.0.0.1")
	t.Setenv("POSTGRES_PORT", "")
	writeEnvFile(t, "POSTGRES_PASSWORD", ".env.development", "POSTGRES_PASSWORD=from_dev\n")
	writeEnvFile(t, "POSTGRES_PASSWORD", ".env.local", "POSTGRES_PASSWORD=from_local\n")
	writeEnvFile(t, "POSTGRES_PASSWORD", ".env.development.local", "POSTGRES_PASSWORD=from_dev_local\n")

	cfg := Load()

	if cfg.Postgres.Password != "from_dev_local" {
		t.Fatalf("expected .env.development.local to win, got %q", cfg.Postgres.Password)
	}
}

// writeEnvFile 在包工作目录写入临时 .env* 文件，测试结束自动清理文件并清掉 env 副作用。
func writeEnvFile(t *testing.T, key, name, content string) {
	t.Helper()
	t.Cleanup(func() {
		_ = os.Remove(name)
		_ = os.Unsetenv(key)
	})
	if err := os.WriteFile(name, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}
