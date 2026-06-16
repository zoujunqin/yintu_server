package httpserver

import (
	"github.com/gin-gonic/gin"

	"spring-slumber-server/internal/app/feature"
)

// API 版本前缀；按需扩展为 v2、v3 等。
const (
	APIPrefixV1 = "/api/v1"
	// APIPrefixV2 = "/api/v2"
)

// RouterGroup 路由分组：同一前缀 + 自己的中间件链。
//
// 设计目的：让不同 API 版本可以挂不同的中间件（鉴权、限流、版本标记等）；
// 同时让业务 feature 无需感知前缀字符串，只声明「我要在 /xxx 注册 handler」。
//
// 使用示例：
//
//	v1 := NewRouterGroup(engine, APIPrefixV1, v1AuthMiddleware, v1LogMiddleware)
//	// feature 内部：
//	v1.Handle("POST", "/user/login", h.Login)   // 实际注册为 POST /api/v1/user/login
type RouterGroup struct {
	group *gin.RouterGroup
}

// NewRouterGroup 构造一个路由分组。
//
// engine：Gin 引擎（顶层 *gin.Engine）。
// prefix：URL 前缀（如 "/api/v1"）。
// middlewares：本组共享的中间件（从外到内依次进入）。
func NewRouterGroup(engine *gin.Engine, prefix string, middlewares ...gin.HandlerFunc) *RouterGroup {
	return &RouterGroup{
		group: engine.Group(prefix, middlewares...),
	}
}

// Prefix 返回组前缀（用于日志/调试）。
func (g *RouterGroup) Prefix() string { return g.group.BasePath() }

// Handle 注册一条 method + path 路由；path 是相对路径（不含 prefix）。
//
// 内部会拼成 method + " " + prefix + path 后注册到分组路由上。
// Gin 路径参数（如 "/user/:id"）可直接写进 path。
func (g *RouterGroup) Handle(method, path string, h gin.HandlerFunc) {
	g.group.Handle(method, path, h)
}

// 编译期断言：RouterGroup 必须实现 feature.Router。
var _ feature.Router = (*RouterGroup)(nil)
