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
func (a *App) Mount(router feature.Router) {
	for _, m := range a.modules {
		m.MountRoutes(router)
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
