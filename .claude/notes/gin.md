---
name: gin
description: Gin v1.10+ 框架核心 API 与最佳实践（路由、中间件、绑定、验证、错误处理、优雅关停）
metadata:
  type: reference
---

# Gin v1.10+ 框架速查

> 来源：[gin-gonic.com/en/docs](https://gin-gonic.com/en/docs/)、[github.com/gin-gonic/gin README](https://github.com/gin-gonic/gin)、[gin-gonic.com/docs/examples/](https://gin-gonic.com/en/docs/examples/)
> 本项目使用 **gin v1.12.0**。

---

## 1. 引擎初始化

### 1.1 `gin.New()` vs `gin.Default()`

```go
gin.SetMode(gin.ReleaseMode)  // 生产必须；默认 DebugMode 会双倍输出路由表
r := gin.New()                // 不带任何中间件
r.Use(Recoverer, Logger, CORS) // 自己精确控制链
```

- `gin.Default()` = `gin.New() + Logger() + Recovery()`，调试期方便，但 Logger 输出格式不可控（不是 slog），生产不推荐。
- **生产推荐**：`gin.New()` + 自定义 slog 适配的 `RequestLogger`。

### 1.2 模式

- `gin.DebugMode`（默认）
- `gin.ReleaseMode`（生产）
- `gin.TestMode`（测试）

由环境变量 `GIN_MODE` 控制：export `GIN_MODE=release`。

---

## 2. 路由注册

来源：[docs/routes-use.md](https://gin-gonic.com/en/docs/routes-use/)

### 2.1 方法快捷

```go
r.GET("/path", handler)
r.POST("/path", handler)
r.PUT("/path", handler)
r.DELETE("/path", handler)
r.PATCH("/path", handler)
r.HEAD("/path", handler)
r.OPTIONS("/path", handler)

r.Any("/any", handler)  // 任意方法
```

### 2.2 路径参数

```go
r.GET("/users/:id", func(c *gin.Context) {
    id := c.Param("id")           // 字符串
})
r.GET("/files/*filepath", ...)    // 通配，c.Param("filepath") 含前导斜杠
```

### 2.3 路由分组（RouterGroup）

```go
v1 := r.Group("/api/v1", AuthMiddleware())
{
    users := v1.Group("/users")
    users.GET("", list)
    users.POST("", create)
    users.GET("/:id", get)
}
```

分组中间件只对组内路由生效；多个分组各自挂自己的中间件。

### 2.4 `Handle(method, path, handlers...)`

```go
v1.Handle("GET", "/overview", overviewHandler.Get)  // 适用 method 为变量
```

### 2.5 静态文件

```go
r.Static("/static", "./public")
r.StaticFS("/static", http.Dir("./public"))
r.StaticFile("/favicon.ico", "./favicon.ico")
```

---

## 3. `*gin.Context` 核心 API

来源：[docs/context-use.md](https://gin-gonic.com/en/docs/context-use/)

### 3.1 取值

```go
c.Param("id")              // 路径参数
c.Query("page")            // ?page=1
c.DefaultQuery("size", "20")
c.GetQuery("page")         // (string, bool)
c.QueryArray("ids")        // ?ids=1&ids=2
c.GetHeader("X-Foo")
c.ClientIP()               // 已处理 X-Forwarded-For（依赖 TrustedProxies）
c.Request                  // 标准 *http.Request
```

### 3.2 绑定请求体

```go
type LoginReq struct {
    Phone string `json:"phone" binding:"required,e164"`
    Code  string `json:"code"  binding:"required,len=6"`
}

// 自动根据 Content-Type 选择：
//   application/json → json
//   application/x-www-form-urlencoded → form
//   multipart/form-data → multipart
if err := c.ShouldBindJSON(&req); err != nil {
    response.Problem(c, http.StatusBadRequest, "invalid_payload", err.Error())
    return
}

// 其它：ShouldBindXML / ShouldBindQuery / ShouldBindUri / ShouldBindHeader
```

### 3.3 响应

```go
c.JSON(status, payload)               // 写 JSON，自动 Content-Type
c.String(status, "hello")
c.XML(status, payload)
c.Data(status, contentType, []byte(...))
c.Status(204)                         // 空响应
c.Render(status, render.JSON{Data: v})
c.AbortWithStatus(401)                // 中止链 + 写状态码
c.Redirect(302, "/login")             // 重定向
```

### 3.4 上下文存储

```go
c.Set("uid", uid)                     // 仅本次请求可见
uidAny, exists := c.Get("uid")
uid := uidAny.(int64)
c.MustGet("uid").(int64)              // 不存在会 panic
```

### 3.5 中止 / 链控制

```go
c.Next()           // 调下一个 handler（middleware 内）
c.Abort()          // 不再调下一个
c.IsAborted()      // 是否已中止
```

---

## 4. 验证（validator/v10）

来源：[go-playground/validator](https://github.com/go-playground/validator)

Gin 内置 `binding:"..."` tag 即用 validator/v10。

### 4.1 常用 tag

```go
type X struct {
    Name  string `binding:"required,min=2,max=64"`
    Email string `binding:"required,email"`
    Phone string `binding:"required,e164"`     // 国际格式
    Age   int    `binding:"gte=0,lte=150"`
    UUID  string `binding:"uuid4"`
    Slice []int  `binding:"required,dive,min=1"` // dive 进入切片元素
}
```

### 4.2 错误统一转换

```go
import "github.com/go-playground/validator/v10"

if err := c.ShouldBindJSON(&req); err != nil {
    var ve validator.ValidationErrors
    if errors.As(err, &ve) {
        // 转成 { field: "phone", rule: "required" } 列表
        details := make([]FieldError, 0, len(ve))
        for _, fe := range ve {
            details = append(details, FieldError{
                Field: fe.Field(),
                Rule:  fe.Tag(),
            })
        }
        response.Problem(c, 400, "validation_failed", "请求参数不合法")
        return
    }
    // JSON 解析错误
    response.Problem(c, 400, "invalid_json", err.Error())
    return
}
```

### 4.3 注册自定义验证

```go
import "github.com/gin-gonic/gin/binding"

func RegisterCustomValidators() {
    if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
        _ = v.RegisterValidation("phone_cn", phoneCNValidator)
    }
}
```

---

## 5. 中间件

来源：[docs/middleware-use.md](https://gin-gonic.com/en/docs/middleware-use/)

### 5.1 定义

`gin.HandlerFunc = func(*gin.Context)`。**所有** 中间件都返回 `gin.HandlerFunc`。

```go
func RequestID() gin.HandlerFunc {
    return func(c *gin.Context) {
        id := c.GetHeader("X-Request-ID")
        if id == "" {
            id = newRequestID()
        }
        c.Writer.Header().Set("X-Request-ID", id)
        c.Set("request_id", id)
        c.Next()  // 调下一环节；不写则请求被吞
    }
}
```

### 5.2 注册

- 全局：`r.Use(M1, M2)`
- 分组：`v1 := r.Group("/v1", M3, M4)`
- 单路由：`r.GET("/x", M5, handler)`

### 5.3 中止写法

```go
if !allowed {
    c.AbortWithStatusJSON(401, gin.H{"error": "unauthorized"})
    return  // 中止后 return 防止继续执行
}
```

### 5.4 自定义 Recovery

Gin 默认 `Recovery()` 把 panic 转 500。要带日志/上报，用：

```go
gin.CustomRecovery(func(c *gin.Context, recovered any) {
    slog.ErrorContext(c.Request.Context(), "panic recovered",
        "error", recovered,
        "stack", string(debug.Stack()),
        "request_id", c.GetString("request_id"),
    )
    response.Problem(c, 500, "internal_error", "Internal server error")
})
```

### 5.5 第三方中间件

```go
import (
    "github.com/gin-contrib/cors"
    "github.com/gin-contrib/requestid"
    "github.com/gin-contrib/zap"
)

r.Use(cors.New(cors.Config{
    AllowOrigins:     []string{"https://example.com"},
    AllowMethods:     []string{"GET", "POST"},
    AllowCredentials: true,
    MaxAge:           12 * time.Hour,
}))
```

---

## 6. 错误处理模式

### 6.1 用 `response.Problem` 统一信封

```go
if err != nil {
    response.Problem(c, http.StatusInternalServerError, "internal_error", "服务异常")
    return
}
```

### 6.2 全局 404 / 405

```go
r.NoRoute(func(c *gin.Context) {
    response.Problem(c, 404, "not_found", "Route not found")
})
r.NoMethod(func(c *gin.Context) {
    response.Problem(c, 405, "method_not_allowed", "Method not allowed")
})
```

### 6.3 链式错误传递

业务层返回错误 → handler 映射状态码：

```go
u, err := h.userSvc.Get(ctx, id)
switch {
case errors.Is(err, dao.ErrUserNotFound):
    response.Problem(c, 404, "user_not_found", "用户不存在")
    return
case err != nil:
    slog.ErrorContext(ctx, "get user failed", "error", err, "uid", id)
    response.Problem(c, 500, "internal_error", "Internal server error")
    return
}
response.JSON(c, 200, u)
```

---

## 7. 优雅关停 + HTTP 服务

来源：[docs/run-server.md](https://gin-gonic.com/en/docs/run-server/)

```go
srv := &http.Server{
    Addr:    ":8080",
    Handler: r,
    // ... 超时配置
}

go func() { _ = srv.ListenAndServe() }()

ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()
<-ctx.Done()

ctx2, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
_ = srv.Shutdown(ctx2)
```

---

## 8. 性能与坑

### 8.1 不要在 handler 里 `c.Request.Body` 重复读取

Body 是流，读完即空。需多次读：

```go
body, _ := io.ReadAll(c.Request.Body)
c.Request.Body = io.NopCloser(bytes.NewReader(body))
```

### 8.2 不要在 handler 里 `time.Sleep`

会阻塞 worker；用 `time.NewTimer + ctx.Done()` 实现。

### 8.3 反向代理后 `c.ClientIP()` 不对？

必须配 TrustedProxies，否则 Gin 1.10+ 默认仅信任直连网段：

```go
r.SetTrustedProxies([]string{"10.0.0.0/8", "127.0.0.1"})
```

### 8.4 JSON binding 用 `ShouldBind` 还是 `MustBind`？

- `Should*`：返回 error，**业务 handler 必用**。
- `Must*`：内部自动 abort+400，适合 framework 层默认 handler。

### 8.5 gzip / TrustedPlatform

```go
r.TrustedPlatform = gin.PlatformCloudflare  // 让 c.ClientIP 取 CF-Connecting-IP
```

---

## 9. 测试

来源：[docs/testing.md](https://gin-gonic.com/en/docs/testing/)

```go
func TestHealth(t *testing.T) {
    gin.SetMode(gin.TestMode)
    r := gin.New()
    r.GET("/healthz", func(c *gin.Context) {
        c.JSON(200, gin.H{"status": "ok"})
    })

    req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)

    if w.Code != 200 {
        t.Fatalf("got %d, want 200", w.Code)
    }
}
```

`TestMode` 关闭日志输出，让测试输出干净。

---

## 10. gin-contrib 生态

| 包 | 用途 |
|---|---|
| `gin-contrib/cors` | CORS 中间件（生产推荐） |
| `gin-contrib/requestid` | 自动请求 ID |
| `gin-contrib/zap` / `slog` | 日志中间件 |
| `gin-contrib/gzip` | 压缩 |
| `gin-contrib/static` | 静态资源 + SPA fallback |
| `gin-contrib/cache` | HTTP 缓存 |
| `gin-contrib/timeout` | handler 超时 |
| `gin-contrib/secure` | 安全 header |
| `gin-contrib/sessions` | Cookie/Redis session |

---

## 11. 官方推荐的反模式清单

| ❌ | ✅ |
|---|---|
| `gin.Default()` 直接上生产 | `gin.New()` + 自定义中间件 |
| 在 handler 里 panic | 用 `CustomRecovery` 兜底 |
| `c.MustGet` 在生产代码用 | 用 `c.Get` + 显式判断 |
| 路由里 `:id` 后直接 `Atoi` 不带校验 | binding tag 校验后再用 |
| 把 ctx 写在全局 map | 只在 `c.Set` / `c.Request.Context()` |
| handler 业务里 `goroutine` 不监听 ctx | 监听 `ctx.Done()` |

---

## 相关资料链接

- [Gin 官方文档首页](https://gin-gonic.com/en/docs/)
- [Gin GitHub README](https://github.com/gin-gonic/gin/blob/master/README.md)
- [gin-gonic/examples](https://github.com/gin-gonic/examples)
- [validator/v10](https://github.com/go-playground/validator)
