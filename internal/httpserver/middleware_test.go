package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"spring-slumber-server/internal/config"
)

func init() {
	// 测试时关闭 Gin 的调试日志，避免污染测试输出。
	gin.SetMode(gin.TestMode)
}

// newCORSRequest 构造一个携带 Origin 的测试请求。
//
// path 是相对路径，Gin 引擎会忽略 host，因此无需写成完整 URL。
func newCORSRequest(method, path, origin string) *http.Request {
	r := httptest.NewRequest(method, path, nil)
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	return r
}

// newOKEngine 构造「CORS + 下游 200 处理器」的引擎。
//
// 注意：Gin 的中间件必须先 Use 再注册路由，因此 middlewares 在此处显式传入，
// 测试用例确保调用顺序正确。
func newOKEngine(path string, middlewares ...gin.HandlerFunc) (*gin.Engine, *bool) {
	called := false
	r := gin.New()
	r.Use(middlewares...)
	r.GET(path, func(c *gin.Context) {
		called = true
		c.Status(http.StatusOK)
	})
	return r, &called
}

// newPreflightEngine 构造「CORS + 注册了 OPTIONS 处理器」的引擎，
// 用于断言「预检是否被 CORS 中间件截断」。
func newPreflightEngine(path string, middlewares ...gin.HandlerFunc) (*gin.Engine, *bool) {
	called := false
	r := gin.New()
	r.Use(middlewares...)
	r.OPTIONS(path, func(c *gin.Context) {
		called = true
		c.Status(http.StatusOK)
	})
	return r, &called
}

func TestCORS_AllowedOrigin_GET(t *testing.T) {
	cfg := config.CORSConfig{
		AllowedOrigins: []string{"http://127.0.0.1:3000"},
		AllowedMethods: []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders: []string{"Authorization", "Content-Type"},
	}

	r, _ := newOKEngine("/api/v1/overview", CORS(cfg))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, newCORSRequest(http.MethodGet, "/api/v1/overview", "http://127.0.0.1:3000"))

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://127.0.0.1:3000" {
		t.Fatalf("expected allow-origin=127.0.0.1:3000, got %q", got)
	}
	if got := rec.Header().Get("Vary"); !strings.Contains(got, "Origin") {
		t.Fatalf("expected Vary to contain Origin, got %q", got)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from downstream, got %d", rec.Code)
	}
}

func TestCORS_AllowedOrigin_PreflightReturns204(t *testing.T) {
	cfg := config.CORSConfig{
		AllowedOrigins: []string{"http://127.0.0.1:3000"},
		AllowedMethods: []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders: []string{"Authorization", "Content-Type"},
		MaxAge:         5 * time.Minute,
	}

	r, called := newPreflightEngine("/api/v1/overview", CORS(cfg))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, newCORSRequest(http.MethodOptions, "/api/v1/overview", "http://127.0.0.1:3000"))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for preflight, got %d", rec.Code)
	}
	if *called {
		t.Fatalf("preflight should short-circuit before downstream handler")
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Fatalf("expected Access-Control-Allow-Methods to be set")
	}
	if got := rec.Header().Get("Access-Control-Max-Age"); got == "" {
		t.Fatalf("expected Access-Control-Max-Age to be set")
	}
}

func TestCORS_DisallowedOrigin_NoHeaders(t *testing.T) {
	cfg := config.CORSConfig{
		AllowedOrigins: []string{"http://allowed.example"},
		AllowedMethods: []string{"GET", "OPTIONS"},
	}

	r, _ := newOKEngine("/api/v1/overview", CORS(cfg))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, newCORSRequest(http.MethodGet, "/api/v1/overview", "http://evil.example"))

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no allow-origin header, got %q", got)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("downstream should still run for non-preflight, got %d", rec.Code)
	}
}

func TestCORS_WildcardWithoutCredentials(t *testing.T) {
	cfg := config.CORSConfig{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET"},
		AllowCredentials: false,
	}

	r, _ := newOKEngine("/api/v1/overview", CORS(cfg))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, newCORSRequest(http.MethodGet, "/api/v1/overview", "http://anywhere"))

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("expected allow-origin=*, got %q", got)
	}
}

func TestCORS_WildcardWithCredentials_ReflectsOrigin(t *testing.T) {
	cfg := config.CORSConfig{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET"},
		AllowCredentials: true,
	}

	r, _ := newOKEngine("/api/v1/overview", CORS(cfg))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, newCORSRequest(http.MethodGet, "/api/v1/overview", "http://127.0.0.1:3000"))

	// 与凭据共存时，浏览器要求必须是"具体 origin"，不能是 "*"。
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://127.0.0.1:3000" {
		t.Fatalf("expected echoed origin when credentials=true, got %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("expected Allow-Credentials=true, got %q", got)
	}
}

// TestRouterGroup_NestedPrefix 验证 RouterGroup.Group() 能正确拼接父 + 子前缀，
// 并让子组的中间件（ClientMarker）正常生效。
//
// 这条用例是子组架构的端到端契约：
//   - 父组前缀 /api/v1 + 子组前缀 /mobile = 实际路径 /api/v1/mobile/user/ping
//   - 子组挂的 ClientMarker 中间件必须把 platform 写入 gin.Context
//   - handler 能读到该标记
func TestRouterGroup_NestedPrefix(t *testing.T) {
	r := gin.New()
	v1 := NewRouterGroup(r, APIPrefixV1)
	mobile := v1.Group("/mobile", ClientMarker("mobile"))

	mobile.Handle("GET", "/user/ping", func(c *gin.Context) {
		platform := c.GetString(ClientPlatformKey)
		c.JSON(http.StatusOK, gin.H{"platform": platform})
	})

	// 命中子组路径
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/mobile/user/ping", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for nested route, got %d, body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"mobile"`) {
		t.Fatalf("expected platform=mobile in body, got %s", rec.Body.String())
	}

	// 父组单独路径不影响子组 —— 这是分组的核心契约
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/api/v1/mobile/user/missing", nil))
	if rec2.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown nested path, got %d", rec2.Code)
	}

	// 父组的路径不在子组上 —— /api/v1/user/ping 没注册，应该 404
	rec3 := httptest.NewRecorder()
	r.ServeHTTP(rec3, httptest.NewRequest(http.MethodGet, "/api/v1/user/ping", nil))
	if rec3.Code != http.StatusNotFound {
		t.Fatalf("expected 404 at parent (no /user/ping registered), got %d", rec3.Code)
	}
}
