package httpserver

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	httpSwagger "github.com/swaggo/http-swagger"

	"spring-slumber-server/internal/app"
	"spring-slumber-server/internal/config"
	_ "spring-slumber-server/internal/docs" // 由 `make swag` 生成；包含 OpenAPI JSON 注册逻辑。
	"spring-slumber-server/internal/handler"
)

// Deps 聚合路由所需的依赖。当前唯一依赖是 app.App，
// 新增 feature 路由无需修改本结构。
type Deps struct {
	App *app.App
}

// newRouter 构造 Gin 引擎并注册所有路由。
//
// 中间件链（外层 → 内层）：CORS → Recoverer → RequestLogger → RequestID → 业务 handler。
// CORS 处于最外层，确保 OPTIONS 预检可以在任何业务逻辑前被截断。
func newRouter(cfg config.Config, logger *slog.Logger, deps Deps) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	r.Use(Recoverer(logger))
	r.Use(RequestLogger(logger))
	r.Use(RequestID())
	r.Use(CORS(cfg.CORS))

	// —— 公共路由（无版本号：欢迎页、健康检查、Swagger UI）——
	healthHandler := handler.NewHealthHandler(cfg)
	r.GET("/", healthHandler.Welcome)
	r.GET("/healthz", healthHandler.Healthz)
	r.GET("/readyz", healthHandler.Readyz)

	// Swagger UI 与 OpenAPI JSON。
	//   - GET /swagger/index.html  Swagger UI 页面
	//   - GET /swagger/doc.json    OpenAPI 规范（前端可直接拉取）
	//   - GET /docs/*              与 /swagger/* 等价的别名
	r.GET("/swagger/*any", gin.WrapH(httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"), // 浏览器相对路径，避免写死 host。
	)))
	r.GET("/docs/*any", gin.WrapH(httpSwagger.Handler(
		httpSwagger.URL("/docs/doc.json"),
	)))

	// —— v1 业务路由分组 ——
	// 后续如加 v2：再 NewRouterGroup(engine, APIPrefixV2, v2Middleware...) + 同样调用 deps.App.Mount。
	v1 := NewRouterGroup(r, APIPrefixV1)
	overviewHandler := handler.NewOverviewHandler(cfg)
	v1.Handle("GET", "/overview", overviewHandler.Get) // GET /api/v1/overview
	deps.App.Mount(v1)

	return r
}
