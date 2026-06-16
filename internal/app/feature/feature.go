// Package feature 定义所有业务 feature 的统一接口。
//
// 设计意图：app.go / router.go 只面向 feature.Module 编程，
// 不感知具体 feature 的内部 DAO/Service 装配；新增 feature 只需
// 实现 Module +（可选）ModelProvider，app 层在 modules.go 中加一行。
package feature

import "github.com/gin-gonic/gin"

// Module 一个业务 feature 的对外门面。
type Module interface {
	// Name 模块名（用于日志/监控/路由前缀）。
	Name() string
	// MountRoutes 把本 feature 的路由注册到 router 上（router 由调用方提供）。
	// path 写相对路径即可（如 "/user/login"），前缀由 router 自己拼接。
	MountRoutes(router Router)
}

// Router 路由注册器抽象（由 httpserver.RouterGroup 实现）。
//
// 引入此抽象的目的：让 feature 不依赖具体路由实现也不感知「前缀是什么」，
// 方便后续加 v2 路由分组时复用同一组 feature。
type Router interface {
	// Handle 注册一条 method+path 路由。
	// path 是相对路径（不含前缀），前缀由实现方拼接。
	Handle(method, path string, h gin.HandlerFunc)
}

// ModelProvider 可选接口：暴露本 feature 的 GORM model 集合，
// 供 app 层统一 AutoMigrate 使用。无 model 的 feature 不实现即可。
//
// 之所以不直接放进 Module：保持 Module 接口最小，
// 同时让"有没有 model"成为可选能力，符合 Go 小接口惯例。
type ModelProvider interface {
	Models() []any
}
