// Package user 实现 user feature 的对外门面（Module）。
//
// 设计意图：app.go 不感知 user 内部的 DAO/Service 装配细节；
// New() 内部完成所有自包含装配（DAO、Service、Handler），
// 外部仅需调用一次 New + MountRoutes 即可。
package user

import (
	"log/slog"

	"gorm.io/gorm"

	"spring-slumber-server/internal/app/feature"
	"spring-slumber-server/internal/app/user/dao"
	"spring-slumber-server/internal/app/user/handler"
	"spring-slumber-server/internal/app/user/model"
	"spring-slumber-server/internal/app/user/service"
	"spring-slumber-server/internal/auth"
	"spring-slumber-server/internal/config"
	"spring-slumber-server/internal/service/ratelimit"
	"spring-slumber-server/internal/service/sms"
)

// Module user feature 的 Module 实现。
type Module struct {
	svc     *service.Service
	handler *handler.Handler
}

// New 构造 user feature 模块。
//
// 内部完成所有 DAO/Service/Handler 装配；外部调用方只需 New 一次。
func New(pg *gorm.DB, cfg config.Config, logger *slog.Logger, jwtIssuer *auth.Issuer) *Module {
	// 1) DAO
	userDAO := dao.NewUserDAO(pg)
	codeDAO := dao.NewVerificationCodeDAO(pg)

	// 2) 基础组件（限流、短信）—— 横切关注点
	limiter := ratelimit.NewMemoryLimiter()
	smsSender := sms.NewSender(cfg.SMS, logger)

	// 3) 业务 Service
	svc := service.NewService(cfg, logger, userDAO, codeDAO, smsSender, limiter, jwtIssuer)

	// 4) Handler
	return &Module{
		svc:     svc,
		handler: handler.NewHandler(svc, logger),
	}
}

// Models 满足 feature.ModelProvider：返回本 feature 需要 AutoMigrate 的所有 model。
func (m *Module) Models() []any {
	return []any{
		&model.User{},
		&model.VerificationCode{},
	}
}

// Name 满足 feature.Module。
func (m *Module) Name() string { return "user" }

// MountRoutes 满足 feature.Module：path 写相对路径，前缀由 router 拼。
func (m *Module) MountRoutes(r feature.Router) {
	r.Handle("POST", "/user/send-code", m.handler.SendCode)
	r.Handle("POST", "/user/login", m.handler.Login)
}
