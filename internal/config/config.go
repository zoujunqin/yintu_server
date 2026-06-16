package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	App      AppConfig
	HTTP     HTTPConfig
	CORS     CORSConfig
	Postgres PostgresConfig
	JWT      JWTConfig
	Auth     AuthConfig
	SMS      SMSConfig
}

type AppConfig struct {
	Name    string
	Env     string
	Version string
}

type PostgresConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	Database        string
	SSLMode         string
	Timezone        string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	DSN             string
}

func (p PostgresConfig) Enabled() bool {
	return p.Host != "" || p.DSN != ""
}

type HTTPConfig struct {
	Host         string
	Port         int
	Addr         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAge           time.Duration
}

// JWTConfig JWT 配置。
type JWTConfig struct {
	Secret    string
	Issuer    string
	ExpiresIn time.Duration
}

// AuthConfig 登录与风控配置。
type AuthConfig struct {
	CodeTTL           time.Duration // 验证码有效期
	CodeSalt          string        // 验证码哈希盐
	SendCodeCooldown  time.Duration // 发送冷却（同 IP / 同手机号）
	LoginIPRateLimit  int           // 单 IP 1 分钟内最多登录请求次数
	LoginIPRateWindow time.Duration // 登录 IP 限流窗口
	MaxVerifyAttempts int           // 单手机号连续错误次数上限
	LockDuration      time.Duration // 账号锁定时长
}

// SMSConfig 短信下发配置。
type SMSConfig struct {
	Provider   string // mock | aliyun | tencent ...
	APIKey     string
	APISecret  string
	SignName   string
	TemplateID string
}

func Load() Config {
	env := envString("APP_ENV", "development")
	// 加载优先级（高 → 低）：
	//   1) 进程环境变量（部署平台/Shell 注入，永远胜出）
	//   2) .env.{APP_ENV}.local  环境特定本地覆盖（已被 .gitignore 忽略）
	//   3) .env.local            本地通用覆盖（test 环境跳过，避免污染）
	//   4) .env.{APP_ENV}        环境特定配置
	//   5) .env                  通用兜底
	//
	// 按"低 → 高"顺序加载，后加载的文件会覆盖先加载的同名键；
	// 进程环境变量由 protected 集合保护，永远不会被 .env 文件覆盖。
	protected := snapshotProcessEnv()
	loadDotEnv(".env", protected)
	loadDotEnv(".env."+env, protected)
	if env != "test" {
		loadDotEnv(".env.local", protected)
	}
	loadDotEnv(".env."+env+".local", protected)

	host := envString("HTTP_HOST", "0.0.0.0")
	port := envInt("HTTP_PORT", 8080)

	return Config{
		App: AppConfig{
			Name:    envString("APP_NAME", "spring-slumber-server"),
			Env:     env,
			Version: envString("APP_VERSION", "dev"),
		},
		HTTP: HTTPConfig{
			Host:         host,
			Port:         port,
			Addr:         net.JoinHostPort(host, strconv.Itoa(port)),
			ReadTimeout:  envDuration("HTTP_READ_TIMEOUT", 5*time.Second),
			WriteTimeout: envDuration("HTTP_WRITE_TIMEOUT", 10*time.Second),
			IdleTimeout:  envDuration("HTTP_IDLE_TIMEOUT", 60*time.Second),
		},
		CORS: CORSConfig{
			AllowedOrigins:   envCSV("CORS_ALLOWED_ORIGINS", []string{"*"}),
			AllowedMethods:   envCSV("CORS_ALLOWED_METHODS", []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}),
			AllowedHeaders:   envCSV("CORS_ALLOWED_HEADERS", []string{"Authorization", "Content-Type", "Accept", "Accept-Language", "Origin", "X-Requested-With", "X-Request-ID"}),
			ExposedHeaders:   envCSV("CORS_EXPOSED_HEADERS", []string{"X-Request-ID"}),
			AllowCredentials: envBool("CORS_ALLOW_CREDENTIALS", false),
			MaxAge:           envDuration("CORS_MAX_AGE", 5*time.Minute),
		},
		Postgres: loadPostgresConfig(),
		JWT: JWTConfig{
			Secret:    envString("JWT_SECRET", "change-me-in-production"),
			Issuer:    envString("JWT_ISSUER", "spring-slumber-server"),
			ExpiresIn: envDuration("JWT_EXPIRES_IN", 2*time.Hour),
		},
		Auth: AuthConfig{
			CodeTTL:           envDuration("AUTH_CODE_TTL", 5*time.Minute),
			CodeSalt:          envString("AUTH_CODE_SALT", "spring-slumber-code-salt"),
			SendCodeCooldown:  envDuration("AUTH_SEND_COOLDOWN", 1*time.Minute),
			LoginIPRateLimit:  envInt("AUTH_LOGIN_IP_LIMIT", 5),
			LoginIPRateWindow: envDuration("AUTH_LOGIN_IP_WINDOW", 1*time.Minute),
			MaxVerifyAttempts: envInt("AUTH_MAX_VERIFY_ATTEMPTS", 5),
			LockDuration:      envDuration("AUTH_LOCK_DURATION", 10*time.Minute),
		},
		SMS: SMSConfig{
			Provider:   envString("SMS_PROVIDER", "mock"),
			APIKey:     envString("SMS_API_KEY", ""),
			APISecret:  envString("SMS_API_SECRET", ""),
			SignName:   envString("SMS_SIGN_NAME", ""),
			TemplateID: envString("SMS_TEMPLATE_ID", ""),
		},
	}
}

func loadPostgresConfig() PostgresConfig {
	dsn := envString("DATABASE_URL", "")
	if dsn == "" {
		host := envString("POSTGRES_HOST", "")
		if host == "" {
			return PostgresConfig{}
		}

		user := envString("POSTGRES_USER", "postgres")
		password := envString("POSTGRES_PASSWORD", "")
		db := envString("POSTGRES_DB", "postgres")
		port := envInt("POSTGRES_PORT", 5432)
		sslmode := envString("POSTGRES_SSLMODE", "disable")
		timezone := envString("POSTGRES_TIMEZONE", "Asia/Shanghai")

		dsn = fmt.Sprintf(
			"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s TimeZone=%s",
			host, port, user, password, db, sslmode, timezone,
		)
	}

	return PostgresConfig{
		Host:            envString("POSTGRES_HOST", ""),
		Port:            envInt("POSTGRES_PORT", 5432),
		User:            envString("POSTGRES_USER", "postgres"),
		Password:        envString("POSTGRES_PASSWORD", ""),
		Database:        envString("POSTGRES_DB", "postgres"),
		SSLMode:         envString("POSTGRES_SSLMODE", "disable"),
		Timezone:        envString("POSTGRES_TIMEZONE", "Asia/Shanghai"),
		MaxOpenConns:    envInt("POSTGRES_MAX_OPEN_CONNS", 25),
		MaxIdleConns:    envInt("POSTGRES_MAX_IDLE_CONNS", 5),
		ConnMaxLifetime: envDuration("POSTGRES_CONN_MAX_LIFETIME", 30*time.Minute),
		ConnMaxIdleTime: envDuration("POSTGRES_CONN_MAX_IDLE_TIME", 5*time.Minute),
		DSN:             dsn,
	}
}

func envString(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func envCSV(key string, fallback []string) []string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			result = append(result, item)
		}
	}

	if len(result) == 0 {
		return fallback
	}

	return result
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	switch value {
	case "":
		return fallback
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}
