// Package main 是 Spring Slumber 服务端入口。
//
// @title                       Spring Slumber API
// @version                     dev
// @description                 Spring Slumber 后端 REST 接口文档（手机号 + 短信验证码登录）
// @BasePath                    /
// @schemes                     http https
// @securityDefinitions.apikey  BearerAuth
// @in                          header
// @name                        Authorization
// @description                 形如 `Bearer <jwt>`，由 POST /api/v1/user/login 返回。
package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"spring-slumber-server/internal/app"
	"spring-slumber-server/internal/config"
	"spring-slumber-server/internal/db"
	"spring-slumber-server/internal/httpserver"
	"spring-slumber-server/internal/security"
)

func main() {
	cfg := config.Load()
	logger := newLogger(cfg.App.Env)

	// 接口加签 + 加解密 RSA keypair：SIGN_PRIVATE_KEY 缺省时自动生成（仅 dev）。
	keyPair := httpserver.LoadKeyPair(cfg, logger)

	pg, err := openPostgres(cfg, logger)
	if err != nil {
		logger.Error("postgres init failed", "error", err)
		os.Exit(1)
	}
	defer func() {
		if pg != nil {
			if err := pg.Close(); err != nil {
				logger.Error("postgres close failed", "error", err)
			}
		}
	}()

	a, err := app.New(cfg, logger, pg)
	if err != nil {
		logger.Error("app init failed", "error", err)
		os.Exit(1)
	}

	runHTTPServer(cfg, logger, a, keyPair)
}

// openPostgres 打开 PG；未配置时返回 (nil, nil) 走无 DB 模式。
func openPostgres(cfg config.Config, logger *slog.Logger) (*db.Postgres, error) {
	startupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pg, err := db.OpenPostgres(startupCtx, cfg.Postgres, logger)
	if err != nil {
		if errors.Is(err, db.ErrPostgresNotConfigured) {
			return nil, nil
		}
		return nil, err
	}
	return pg, nil
}

// runHTTPServer 启动 HTTP 服务并阻塞到收到退出信号。
func runHTTPServer(cfg config.Config, logger *slog.Logger, a *app.App, keyPair *security.KeyPair) {
	server := httpserver.New(cfg, logger, httpserver.Deps{App: a, KeyPair: keyPair})
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Start()
	}()
	logger.Info("server started", "addr", cfg.HTTP.Addr, "env", cfg.App.Env)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("server shutdown failed", "error", err)
		}
		logger.Info("server stopped")
	case err := <-errCh:
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func newLogger(env string) *slog.Logger {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	if env == "production" {
		return slog.New(slog.NewJSONHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, opts))
}
