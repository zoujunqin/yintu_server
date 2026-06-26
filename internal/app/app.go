// Package app 集中装配所有业务 feature，对外只暴露 App 一个类型。
//
// 设计原则：
//   - main.go / httpserver / router.go 只面向 app.App 编程。
//   - 业务 feature 全部放在 app/<feature>/，自包含 DAO/Service 装配。
//   - 加新 feature：新建 feature 包 + 在 modules.go 加一行（无需改 app.go 本体）。
//   - 后续如需引入 wire/fx，先评估 newModules 是否仍可一目了然。
package app

import (
	"log/slog"

	"gorm.io/gorm"

	"spring-slumber-server/internal/app/feature"
	"spring-slumber-server/internal/config"
	"spring-slumber-server/internal/db"
)

// App 聚合所有业务模块，对外只暴露 Mount 一个动作。
type App struct {
	cfg     config.Config
	logger  *slog.Logger
	pg      *db.Postgres
	modules []feature.Module
}

// New 构造 App。
//
// pg == nil 时返回的 App 业务模块为空，Mount 时不会注册任何业务路由。
func New(cfg config.Config, logger *slog.Logger, pg *db.Postgres) (*App, error) {
	a := &App{cfg: cfg, logger: logger, pg: pg}
	if pg == nil {
		logger.Warn("postgres not configured, business modules disabled")
		return a, nil
	}

	// 1) 装载各 feature（在内部完成 DAO/Service 装配）。
	a.modules = newModules(pg.DB, cfg, logger)

	// 2) 一次性 AutoMigrate。
	if err := a.migrate(pg.DB); err != nil {
		return nil, err
	}
	return a, nil
}

// Mount 把所有 feature 的路由注册到 router 上（router 由调用方提供，通常是 httpserver.RouterGroup）。
//
// 这是「公共路由」入口：不区分客户端，所有 feature 的 MountRoutes 都会挂到 router 上。
// 若需要在 v1 下再分客户端子组（移动端 / 管理端 / 用户端），由调用方在 router.go
// 用 router.Group(...) 创建子组后，再分别调用下面的 MountMobile / MountAdmin / MountPCUser。
func (a *App) Mount(router feature.Router) {
	for _, m := range a.modules {
		m.MountRoutes(router)
	}
}

// MountMobile 把所有实现了 feature.MobileProvider 的 feature 挂到 mobile 子组上。
//
// 没实现该接口的 feature 会被静默跳过 —— 这是设计意图：
// 加新 feature 时默认不挂任何客户端子组，按需实现接口即可。
func (a *App) MountMobile(router feature.Router) {
	for _, m := range a.modules {
		if p, ok := m.(feature.MobileProvider); ok {
			p.MountMobileRoutes(router)
		}
	}
}

// MountAdmin 把所有实现了 feature.AdminProvider 的 feature 挂到 admin 子组上。
func (a *App) MountAdmin(router feature.Router) {
	for _, m := range a.modules {
		if p, ok := m.(feature.AdminProvider); ok {
			p.MountAdminRoutes(router)
		}
	}
}

// MountPCUser 把所有实现了 feature.PCUserProvider 的 feature 挂到 pc-user 子组上。
func (a *App) MountPCUser(router feature.Router) {
	for _, m := range a.modules {
		if p, ok := m.(feature.PCUserProvider); ok {
			p.MountPCUserRoutes(router)
		}
	}
}

// migrate 收集所有 feature 暴露的 model，调用 AutoMigrate。
func (a *App) migrate(db *gorm.DB) error {
	var models []any
	for _, m := range a.modules {
		if p, ok := m.(feature.ModelProvider); ok {
			models = append(models, p.Models()...)
		}
	}
	if len(models) == 0 {
		return nil
	}
	return db.AutoMigrate(models...)
}
