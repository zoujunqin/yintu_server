---
name: go-stdlib
description: Go 1.22+ 标准库核心 API 速查（net/http、context、log/slog、errors、sync/errgroup、testing）
metadata:
  type: reference
---

# Go 标准库速查（1.22+）

> 来源：[go.dev/doc](https://go.dev/doc)、[pkg.go.dev/std](https://pkg.go.dev/std)、[go.dev/wiki/EffectiveGo](https://go.dev/wiki/EffectiveGo)、[go.dev/blog](https://go.dev/blog/)
> 只收录本项目高频用到的 API 与官方推荐用法。

---

## 1. `net/http` & `http.Server`

来源：[pkg.go.dev/net/http](https://pkg.go.dev/net/http)、[go.dev/doc/articles/wiki](https://go.dev/doc/articles/wiki)

### 1.1 `http.Server` 字段（必须显式设置，否则默认值为 0 = 无超时）

```go
srv := &http.Server{
    Addr:              ":8080",
    Handler:           router,
    ReadTimeout:       10 * time.Second,  // 读整请求（含 body）
    ReadHeaderTimeout: 5 * time.Second,   // 仅读 header；建议 ≤ ReadTimeout
    WriteTimeout:      15 * time.Second,  // 写响应
    IdleTimeout:       60 * time.Second,  // keep-alive 空闲
    BaseContext:       func(net.Listener) context.Context { return ctx }, // 顶层 ctx
}
```

### 1.2 优雅关停（官方惯用法）

```go
ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
defer stop()

go func() { _ = srv.ListenAndServe() }()
<-ctx.Done()

shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
if err := srv.Shutdown(shutdownCtx); err != nil {
    // Shutdown 返回 context.DeadlineExceeded 表示超时仍有连接未关闭
    _ = srv.Close() // 兜底强关
}
```

要点：
- `Shutdown` 先关闭 listener，再等待活跃连接自行结束（受 `shutdownCtx` 超时约束）。
- `Shutdown` 返回的 error 用 `errors.Is(err, context.DeadlineExceeded)` 判断。
- `signal.NotifyContext`（Go 1.16+）替代旧版 `signal.Notify` + 手动 `cancel()`。

### 1.3 Go 1.22+ ServeMux 模式匹配

来源：[go.dev/doc/articles/wiki](https://go.dev/doc/articles/wiki)、Go 1.22 release notes

```go
mux := http.NewServeMux()
mux.HandleFunc("GET /api/v1/users/{id}", getUser)   // 1.22+
mux.HandleFunc("POST /api/v1/users", createUser)
```

要点：
- 方法前缀（`GET`/`POST`/...）+ 精确路径，比第三方路由器快且无需依赖。
- `{id}` 是占位符，`r.PathValue("id")` 取值（替代旧的 `mux.Vars`）。
- 本项目用 Gin 故不直接用，但底层可选。

### 1.4 自定义 `RoundTripper`（如需调外部 API）

```go
tr := &http.Transport{
    MaxIdleConns:        100,
    MaxIdleConnsPerHost: 10,
    IdleConnTimeout:     90 * time.Second,
    TLSHandshakeTimeout: 5 * time.Second,
}
client := &http.Client{Transport: tr, Timeout: 30 * time.Second}
```

---

## 2. `context` 传递规则

来源：[pkg.go.dev/context](https://pkg.go.dev/context)、[go.dev/blog/context](https://go.dev/blog/context)、[go.dev/blog/pipelines](https://go.dev/blog/pipelines)

### 2.1 派生与取消

```go
ctx, cancel := context.WithTimeout(parent, 3*time.Second)
defer cancel()  // 必加，避免 timer 泄漏

// 读链：
ctx.Err()        // 取消原因：Canceled / DeadlineExceeded
ctx.Done()       // <-chan struct{}
ctx.Deadline()   // (time.Time, bool)
```

### 2.2 铁律

- ✅ 所有 IO 函数（DB/HTTP/Redis/RPC）**第一个参数**是 `ctx context.Context`。
- ✅ Handler/Service/DAO 一路透传 ctx，**不要** `context.Background()` 重新起。
- ✅ 只在 main / 启动 goroutine / 单元测试入口用 `context.Background()`。
- ✅ 跨包传值用**自定义非字符串 type key**，避免冲突：
  ```go
  type ctxKey string
  const requestIDKey ctxKey = "request_id"
  ctx.Value(requestIDKey)
  ```
- ❌ 业务字段（uid、user）别放 ctx；ctx 用于「请求级元数据」。
- ❌ 不要把 `*gin.Context` 当 ctx 透传到 DAO（应 `c.Request.Context()`）。

### 2.3 Gin 中的 ctx

Gin `*gin.Context` 自带 `c.Request.Context()`，handler 内调用 DAO 时：

```go
u, err := h.svc.GetUser(c.Request.Context(), userID)
```

---

## 3. `log/slog` 结构化日志

来源：[pkg.go.dev/log/slog](https://pkg.go.dev/log/slog)、[go.dev/blog/slog](https://go.dev/blog/slog)

### 3.1 初始化

```go
opts := &slog.HandlerOptions{Level: slog.LevelInfo}
var h slog.Handler = slog.NewTextHandler(os.Stdout, opts)
if env == "production" {
    h = slog.NewJSONHandler(os.Stdout, opts)
}
logger := slog.New(h).With("service", "spring-slumber", "version", version)
slog.SetDefault(logger)  // 让 gin/gorm 等三方库间接使用
```

### 3.2 调用方式（按性能排序）

```go
// 1. 慢但灵活（fmt 风格，运行时反射）
slog.Info("user login", "uid", uid, "ip", ip)

// 2. 快且类型安全（推荐热路径）
slog.LogAttrs(ctx, slog.LevelInfo, "user login",
    slog.Int64("uid", uid),
    slog.String("ip", ip),
)

// 3. 上下文感知（Go 1.21+；从 ctx 抽 trace_id 等）
slog.InfoContext(ctx, "user login", "uid", uid)
```

### 3.3 With / Group

```go
reqLogger := logger.With("request_id", id, "uid", uid) // 派生不修改原 logger
slog.SetDefault(reqLogger)

// 分组输出 JSON：{"user":{"id":1,"name":"x"}}
logger.Info("ok", slog.Group("user", slog.Int64("id", 1), slog.String("name", "x")))
```

### 3.4 错误日志约定

```go
slog.Error("postgres query failed",
    "error", err,                     // 关键：error 字段名固定
    "elapsed_ms", elapsed.Milliseconds(),
    "sql", sql,
)
```

### 3.5 自定义 Handler（GORM 适配）

实现 `slog.Handler` 接口（`Enabled/Handle/WithAttrs/WithGroup`），比 `slog.NewJSONHandler` 改造更灵活；本项目 GORM 适配器实现的是 GORM 自家的 `logger.Interface`，走的是 Trace/Info/Warn/Error 四方法。

---

## 4. `errors` 包装与哨兵

来源：[pkg.go.dev/errors](https://pkg.go.dev/errors)、[go.dev/blog/go1.13-errors](https://go.dev/blog/go1.13-errors)

### 4.1 包装（必须用 `w`）

```go
if err != nil {
    return fmt.Errorf("postgres ping: %w", err)        // 保留链
}
// 仅展示用：
return fmt.Errorf("postgres ping: %v", err)            // 不保留链
```

### 4.2 哨兵与自定义类型

```go
var ErrUserNotFound = errors.New("user not found")

// 解包
if errors.Is(err, ErrUserNotFound) { ... }

// 类型断言（自定义错误结构时）
var e *MyError
if errors.As(err, &e) { ... }
```

### 4.3 多次包装

```go
return fmt.Errorf("layer A: %w", fmt.Errorf("layer B: %w", err))
// errors.Unwrap 多次调用可逐层解开
```

### 4.4 注意

- ❌ `errors.Is(err, nil)` 恒为 true；判断 nil 直接 `err != nil`。
- ❌ 不要 panic 处理可恢复错误。

---

## 5. `sync` & `errgroup`

来源：[pkg.go.dev/sync](https://pkg.go.dev/sync)、[pkg.go.dev/golang.org/x/sync/errgroup](https://pkg.go.dev/golang.org/x/sync/errgroup)

### 5.1 `errgroup` 适合「并发任务，任一失败即整体取消」

```go
g, gctx := errgroup.WithContext(ctx)
for _, u := range users {
    u := u
    g.Go(func() error {
        return sendEmail(gctx, u)  // 第一个出错则 gctx 自动 cancel
    })
}
if err := g.Wait(); err != nil { return err }
```

### 5.2 goroutine 模板

```go
go func() {
    timer := time.NewTicker(1 * time.Minute)
    defer timer.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case <-timer.C:
            if err := doWork(ctx); err != nil {
                slog.ErrorContext(ctx, "background work failed", "error", err)
            }
        }
    }
}()
```

铁律：goroutine 内必监听 `ctx.Done()`，否则进程关不掉。

---

## 6. `encoding/json`

来源：[pkg.go.dev/encoding/json](https://pkg.go.dev/encoding/json)

### 6.1 struct tag 约定

```go
type User struct {
    ID    int64  `json:"id"`
    Name  string `json:"name,omitempty"`
    Phone string `json:"-"`  // 完全跳过
    Tag   string `json:"tag,string"`  // 把 string 编码为 JSON 数字/字符串
}
```

### 6.2 `json.RawMessage` 透传

```go
var raw json.RawMessage
if err := json.Unmarshal(body, &raw); err != nil { ... }
```

### 6.3 反序列化错误定位

`json.Unmarshal` 的 error 类型是 `*json.SyntaxError`，可读 `Offset` 字段定位。

---

## 7. `testing` 表驱动

来源：[pkg.go.dev/testing](https://pkg.go.dev/testing)

```go
func TestParsePhone(t *testing.T) {
    tests := []struct {
        name    string
        in      string
        wantErr bool
    }{
        {"valid", "13800000000", false},
        {"too short", "123", true},
    }
    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            _, err := ParsePhone(tc.in)
            if (err != nil) != tc.wantErr {
                t.Fatalf("ParsePhone(%q) error=%v, wantErr=%v", tc.in, err, tc.wantErr)
            }
        })
    }
}
```

辅助函数用 `t.Helper()` 让报错行号指向调用方。

---

## 8. `time` 常见坑

- ❌ `time.Now()` 当 key 进 map（不可比较）。
- ✅ 持久化用 UTC（`time.Now().UTC()`），展示再转本地时区。
- ✅ 业务时间用 `time.Duration`（`30 * time.Minute`），不用 int 秒。
- ✅ 解析 RFC3339 用 `time.Parse(time.RFC3339, s)`。

---

## 9. `os/signal` 退出

```go
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()
```

Docker/K8s 场景 SIGTERM 是主流，进程应在收到后 10s 内退出；`TerminationGracePeriodSeconds` 默认 30s。

---

## 10. 官方推荐 vs 反模式

| 场景 | ✅ 官方推荐 | ❌ 反模式 |
|---|---|---|
| 错误处理 | `if err != nil { return ... }` | panic / try-catch-like |
| 字符串拼接 | `fmt.Sprintf` 或 `strings.Builder` | `+` 拼接（除短字符串） |
| 配置 | 环境变量 + `os.Getenv` 或 `flag` | 硬编码常量 |
| 依赖管理 | `go mod tidy`，最小依赖 | `GOPATH` / `vendor` 直拷贝 |
| 并发 | goroutine + channel 或 errgroup | 全局 `sync.Mutex` 锁大块 |
| API 设计 | 接受接口、返回结构体 | 接口膨胀（Java 风格） |
| 日志 | `log/slog` | `log.Print` |
| 测试 | 表驱动 + 子测试 | 串行大量重复 TestXxx |

---

## 相关资料链接

- [Effective Go](https://go.dev/wiki/EffectiveGo)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Standard library](https://pkg.go.dev/std)
- [go.dev/doc](https://go.dev/doc/)
- [go.dev/blog](https://go.dev/blog/)
