package httpserver

import (
	"log/slog"
	"strings"

	"github.com/gin-gonic/gin"
	httpSwagger "github.com/swaggo/http-swagger"

	"spring-slumber-server/internal/app"
	"spring-slumber-server/internal/app/i18n/locale"
	"spring-slumber-server/internal/config"
	_ "spring-slumber-server/internal/docs" // 由 `make swag` 生成；包含 OpenAPI JSON 注册逻辑。
	"spring-slumber-server/internal/handler"
	"spring-slumber-server/internal/security"
)

// Deps 聚合路由所需的依赖。
type Deps struct {
	App     *app.App
	KeyPair *security.KeyPair
}

// isPublicPath 公开端点不走加签 + 加解密中间件。
func isPublicPath(path string) bool {
	if path == "/" || path == "/healthz" || path == "/readyz" {
		return true
	}
	if path == "/security/public-key" {
		return true
	}
	if strings.HasPrefix(path, "/swagger/") || strings.HasPrefix(path, "/docs/") {
		return true
	}
	return false
}

// newRouter 构造 Gin 引擎并注册所有路由。
//
// 中间件链（外层 → 内层）：CORS → Recoverer → RequestLogger → RequestID → Security → 业务 handler。
func newRouter(cfg config.Config, logger *slog.Logger, deps Deps) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	r.Use(Recoverer(logger))
	r.Use(RequestLogger(logger))
	r.Use(RequestID())
	r.Use(locale.Middleware())
	r.Use(CORS(cfg.CORS))

	// —— 公共路由（无版本号：欢迎页、健康检查、安全公钥、Swagger UI）——
	healthHandler := handler.NewHealthHandler(cfg)
	r.GET("/", healthHandler.Welcome)
	r.GET("/healthz", healthHandler.Healthz)
	r.GET("/readyz", healthHandler.Readyz)

	// 服务端 RSA 公钥：供前端首次访问时拉取（也可由 build 期 env 注入）。
	// 永远返回明文 JSON，不过中间件。
	r.GET("/security/public-key", func(c *gin.Context) {
		spki, err := security.MarshalPublicKeySPKI(deps.KeyPair.Public)
		if err != nil {
			c.AbortWithStatusJSON(500, gin.H{"error": gin.H{"code": "internal_error", "message": err.Error()}})
			return
		}
		c.JSON(200, gin.H{
			"kid":  "server-key-1",
			"alg":  "RSA-OAEP-SHA256 / AES-256-GCM / RSA-PSS-SHA256",
			"spki": spki,
			"pem":  mustPublicKeyPEM(deps.KeyPair),
		})
	})

	r.GET("/swagger/*any", gin.WrapH(httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	)))
	r.GET("/docs/*any", gin.WrapH(httpSwagger.Handler(
		httpSwagger.URL("/docs/doc.json"),
	)))

	// —— v1 业务路由分组 —— 走 security 中间件 ———
	v1 := NewRouterGroup(r, APIPrefixV1, security.Middleware(security.Config{
		KeyPair:         deps.KeyPair,
		EncryptDisabled: !cfg.Security.EncryptEnabled,
		Skipper:         func(c *gin.Context) bool { return isPublicPath(c.Request.URL.Path) },
	}))
	overviewHandler := handler.NewOverviewHandler(cfg)
	v1.Handle("GET", "/overview", overviewHandler.Get)

	deps.App.Mount(v1)

	// —— 客户端子组 ——
	mobile := v1.Group("/mobile", ClientMarker("mobile"))
	admin := v1.Group("/pc-admin", ClientMarker("pc-admin"))
	pcUser := v1.Group("/pc-user", ClientMarker("pc-user"))

	deps.App.MountMobile(mobile)
	deps.App.MountAdmin(admin)
	deps.App.MountPCUser(pcUser)

	return r
}

func mustPublicKeyPEM(kp *security.KeyPair) string {
	pem, err := security.PublicKeyPEM(kp.Public)
	if err != nil {
		return ""
	}
	return pem
}
