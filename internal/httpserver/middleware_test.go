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
