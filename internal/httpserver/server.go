package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"spring-slumber-server/internal/config"
	"spring-slumber-server/internal/security"
)

// Server HTTP 服务封装。
//
// 设计说明：内部仍然使用 *http.Server 作为外壳，目的是复用 net/http 的
// ReadTimeout / WriteTimeout / IdleTimeout / Shutdown(ctx) 优雅停机能力。
// 实际请求处理委托给 gin.Engine。
type Server struct {
	httpServer *http.Server
}

// New 构造 HTTP 服务。
func New(cfg config.Config, logger *slog.Logger, deps Deps) *Server {
	return &Server{
		httpServer: &http.Server{
			Addr:         cfg.HTTP.Addr,
			Handler:      newRouter(cfg, logger, deps),
			ReadTimeout:  cfg.HTTP.ReadTimeout,
			WriteTimeout: cfg.HTTP.WriteTimeout,
			IdleTimeout:  cfg.HTTP.IdleTimeout,
		},
	}
}

// LoadKeyPair 从 cfg.Security 加载或生成 RSA keypair；失败则进程退出。
func LoadKeyPair(cfg config.Config, logger *slog.Logger) *security.KeyPair {
	kp, err := security.LoadOrGenerateKeyPair(cfg.Security.SignPrivateKey, cfg.Security.SignPublicKey)
	if err != nil {
		logger.Error("security keypair load failed", "error", err)
		panic(err)
	}
	if cfg.Security.SignPrivateKey == "" {
		logger.Warn("SIGN_PRIVATE_KEY not set, generated ephemeral RSA-2048 keypair (dev only)")
	}
	return kp
}

// Start 阻塞监听；返回值为非 nil 时表示监听异常。
func (s *Server) Start() error {
	if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Shutdown 优雅停机；超时由 ctx 控制。
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
