package app

import (
	"log/slog"

	"gorm.io/gorm"

	"spring-slumber-server/internal/app/admin-user"
	"spring-slumber-server/internal/app/feature"
	"spring-slumber-server/internal/auth"
	"spring-slumber-server/internal/config"
)

// newModules 装载所有 feature 模块。
//
// 加新 feature 的唯一改动点：
//  1. 顶部 import 一行；
//  2. 此处加一行。
//
// 其余 app.go / router.go / main.go 均不动。
func newModules(pg *gorm.DB, cfg config.Config, logger *slog.Logger) []feature.Module {
	// 公共基础组件（横切关注点：JWT 签发器等），按需扩展。
	jwtIssuer := auth.NewIssuer(cfg.JWT)

	return []feature.Module{
		user.New(pg, cfg, logger, jwtIssuer),
		// order.New(pg, cfg, logger, ...),
		// notify.New(pg, cfg, logger, ...),
	}
}
