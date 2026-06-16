// Package db 提供 PostgreSQL 连接初始化与生命周期管理。
//
// 采用 GORM v2 作为 ORM 驱动，全局单例 *gorm.DB，
// 启动时 Ping 校验，Shutdown 时关闭底层 *sql.DB。
package db

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"spring-slumber-server/internal/config"
)

// ErrPostgresNotConfigured 表示未提供 PostgreSQL 连接配置。
var ErrPostgresNotConfigured = errors.New("postgres config is not configured")

// Postgres 持有 *gorm.DB 与关闭函数。
type Postgres struct {
	DB     *gorm.DB
	closer func() error
}

// OpenPostgres 根据配置建立 GORM 连接，并配置连接池。
//
// ctx 用于启动时的 Ping 校验；调用方负责在程序退出时调用 Close。
func OpenPostgres(ctx context.Context, cfg config.PostgresConfig, logger *slog.Logger) (*Postgres, error) {
	if !cfg.Enabled() {
		return nil, ErrPostgresNotConfigured
	}

	gormCfg := &gorm.Config{
		Logger:                                   newSlogAdapter(logger),
		DisableForeignKeyConstraintWhenMigrating: true,
	}

	gormDB, err := gorm.Open(postgres.Open(cfg.DSN), gormCfg)
	if err != nil {
		return nil, fmt.Errorf("gorm open postgres: %w", err)
	}

	sqlDB, err := gormDB.DB()
	if err != nil {
		return nil, fmt.Errorf("gorm get sql.DB: %w", err)
	}

	if cfg.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}
	if cfg.ConnMaxIdleTime > 0 {
		sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(pingCtx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("postgres ping: %w", err)
	}

	logger.Info(
		"postgres connected",
		"host", cfg.Host,
		"port", cfg.Port,
		"database", cfg.Database,
		"max_open_conns", cfg.MaxOpenConns,
		"max_idle_conns", cfg.MaxIdleConns,
	)

	return &Postgres{
		DB:     gormDB,
		closer: sqlDB.Close,
	}, nil
}

// Close 关闭底层 *sql.DB，幂等。
func (p *Postgres) Close() error {
	if p == nil || p.closer == nil {
		return nil
	}
	return p.closer()
}

// slogAdapter 将 slog.Logger 适配到 gorm logger.Interface。
type slogAdapter struct {
	logger        *slog.Logger
	slowThreshold time.Duration
}

func newSlogAdapter(logger *slog.Logger) gormlogger.Interface {
	return &slogAdapter{
		logger:        logger.With("component", "gorm"),
		slowThreshold: 200 * time.Millisecond,
	}
}

func (s *slogAdapter) LogMode(gormlogger.LogLevel) gormlogger.Interface {
	// 直接复用同一 logger；如需切换级别可在此处实现。
	return s
}

func (s *slogAdapter) Info(_ context.Context, msg string, data ...interface{}) {
	s.logger.Info(fmt.Sprint(msg), "data", fmt.Sprint(data...))
}

func (s *slogAdapter) Warn(_ context.Context, msg string, data ...interface{}) {
	s.logger.Warn(fmt.Sprint(msg), "data", fmt.Sprint(data...))
}

func (s *slogAdapter) Error(_ context.Context, msg string, data ...interface{}) {
	s.logger.Error(fmt.Sprint(msg), "data", fmt.Sprint(data...))
}

func (s *slogAdapter) Trace(_ context.Context, begin time.Time, fc func() (string, int64), err error) {
	elapsed := time.Since(begin)
	sql, rows := fc()
	switch {
	case err != nil && !errors.Is(err, gorm.ErrRecordNotFound):
		s.logger.Error("gorm query failed",
			"error", err,
			"elapsed_ms", elapsed.Milliseconds(),
			"rows", rows,
			"sql", sql,
		)
	case elapsed > s.slowThreshold:
		s.logger.Warn("slow gorm query",
			"elapsed_ms", elapsed.Milliseconds(),
			"rows", rows,
			"sql", sql,
		)
	}
}
