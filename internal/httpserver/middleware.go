package httpserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"spring-slumber-server/internal/config"
	"spring-slumber-server/internal/response"
)

// RequestIDKey 是请求 ID 在 *gin.Context 中的键名。
const RequestIDKey = "request_id"

// ctxRequestIDKey 是请求 ID 在 *http.Request.Context() 中的 context key。
// 与 RequestIDKey 保持同步写入，便于 service / DAO 等下游代码直接拿到。
type ctxRequestIDKeyT struct{}

var ctxRequestIDKey = ctxRequestIDKeyT{}

// RequestID 注入 / 透传 X-Request-ID。
//
// 优先级：优先使用上游传入的 X-Request-ID；缺失时生成 16 字节随机十六进制串。
// 同时把请求 ID 写入：
//   - 响应头：X-Request-ID
//   - *gin.Context：c.GetString(RequestIDKey)
//   - *http.Request.Context()：ctx.Value(ctxRequestIDKey)
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-ID")
		if id == "" {
			id = newRequestID()
		}

		c.Writer.Header().Set("X-Request-ID", id)
		c.Set(RequestIDKey, id)
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxRequestIDKey, id))
		c.Next()
	}
}

// RequestLogger 打印结构化访问日志。
//
// 日志字段：method、path、status、duration_ms、request_id、remote_addr、user_agent。
// 注意：CORS 预检（OPTIONS）会被外层 CORS 中间件截断，本中间件不会触发日志，与原 net/http 实现一致。
func RequestLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		logger.Info(
			"http request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", c.GetString(RequestIDKey),
			"remote_addr", c.Request.RemoteAddr,
			"user_agent", c.Request.UserAgent(),
		)
	}
}

// Recoverer 捕获 handler panic，统一返回 500 错误响应。
//
// 日志字段：error、request_id、stack；响应使用统一的 error 信封。
func Recoverer(logger *slog.Logger) gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		logger.Error(
			"panic recovered",
			"error", recovered,
			"request_id", c.GetString(RequestIDKey),
			"stack", string(debug.Stack()),
		)
		response.Problem(c, http.StatusInternalServerError, "internal_error", "Internal server error")
	})
}

// CORS 实现 CORS 预检与响应头注入。
//
// 行为：
//   - 命中白名单的 origin：注入 Access-Control-Allow-* 系列头。
//   - 凭据模式 + 通配 origin：必须回显具体 origin，不能 "*"。
//   - OPTIONS 预检：短路返回 204，不再走到业务 handler。
//
// 命中规则的 origin 在非 OPTIONS 请求上仍放行业务 handler；不命中则不写 CORS 头，
// 由浏览器自动拦截，业务链路不受影响。
func CORS(cfg config.CORSConfig) gin.HandlerFunc {
	allowAllOrigins := slices.Contains(cfg.AllowedOrigins, "*")
	methods := strings.Join(cfg.AllowedMethods, ", ")
	headers := strings.Join(cfg.AllowedHeaders, ", ")
	exposed := strings.Join(cfg.ExposedHeaders, ", ")
	maxAge := int(cfg.MaxAge.Seconds())

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		allowed := isOriginAllowed(origin, cfg.AllowedOrigins, allowAllOrigins)

		if allowed {
			// 注意：当 Allow-Credentials=true 时，不能回显 "*"，
			// 必须把请求里的 Origin 原样回写，否则浏览器会拒绝响应。
			if allowAllOrigins && !cfg.AllowCredentials {
				c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
			} else {
				c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
				c.Writer.Header().Add("Vary", "Origin")
			}

			if methods != "" {
				c.Writer.Header().Set("Access-Control-Allow-Methods", methods)
			}
			if headers != "" {
				c.Writer.Header().Set("Access-Control-Allow-Headers", headers)
			}
			if exposed != "" {
				c.Writer.Header().Set("Access-Control-Expose-Headers", exposed)
			}
			if cfg.AllowCredentials {
				c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			}
			if maxAge > 0 {
				c.Writer.Header().Set("Access-Control-Max-Age", strconv.Itoa(maxAge))
			}
		}

		if c.Request.Method == http.MethodOptions {
			// 预检请求无需再走到业务处理器，统一返回 204。
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// isOriginAllowed 判断 origin 是否在白名单。
func isOriginAllowed(origin string, allowed []string, allowAll bool) bool {
	if origin == "" {
		return false
	}
	if allowAll {
		return true
	}
	return slices.Contains(allowed, origin)
}

// newRequestID 生成 16 字节随机十六进制串；失败时退化到时间戳。
func newRequestID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(bytes[:])
}
