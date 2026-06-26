// Package user 实现 user feature 的对外门面（Module）。
//
// 设计意图：app.go 不感知 user 内部的 DAO/Service 装配细节；
// New() 内部完成所有自包含装配（DAO、Service、Handler），
// 外部仅需调用一次 New + MountRoutes 即可。
package user

import (
	"log/slog"

	"gorm.io/gorm"

	"spring-slumber-server/internal/app/admin-user/dao"
	"spring-slumber-server/internal/app/admin-user/handler"
	"spring-slumber-server/internal/app/admin-user/model"
	"spring-slumber-server/internal/app/admin-user/service"
	"spring-slumber-server/internal/app/feature"
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
//
// 公共路由（不区分客户端）：发送验证码、登录。
func (m *Module) MountRoutes(r feature.Router) {
	r.Handle("POST", "/user/send-code", m.handler.SendCode)
	r.Handle("POST", "/user/login", m.handler.Login)
}

// MountMobileRoutes 满足 feature.MobileProvider（可选接口）：
// 注册移动端子组 (/api/v1/mobile) 下的 user 相关路由。
//
// 当前 demo：移动端连通性探测。
// 后续真实业务：设备指纹上报、推送 token 绑定、退出登录等。
func (m *Module) MountMobileRoutes(r feature.Router) {
	r.Handle("GET", "/user/ping", m.handler.MobilePing)
}

// 编译期断言：*Module 必须实现 feature.Module 与 feature.MobileProvider。
var (
	_ feature.Module         = (*Module)(nil)
	_ feature.MobileProvider = (*Module)(nil)
)
