---
name: swag-openapi
description: swaggo/swag + http-swagger OpenAPI 文档生成与集成 Gin 最佳实践
metadata:
  type: reference
---

# swag / OpenAPI 速查

> 来源：[github.com/swaggo/swag](https://github.com/swaggo/swag)、[github.com/swaggo/http-swagger](https://github.com/swaggo/http-swagger)、[swagger.io spec](https://swagger.io/specification/)
> 本项目使用 **swag v1.16.6** + **http-swagger v1.3.4**。

---

## 1. 安装 CLI

```bash
go install github.com/swaggo/swag/cmd/swag@latest
```

`swag init -g cmd/server/main.go -o internal/docs --parseDependency --parseInternal`

常用参数：

| 参数 | 作用 |
|---|---|
| `-g` | 入口 main 文件 |
| `-o` | 生成目录（默认 `./docs`） |
| `--parseDependency` / `--parseInternal` | 解析依赖/内部包里的注解 |
| `--generatedTime` | 生成时是否带时间戳 |
| `--parseDepth` | 依赖解析深度 |

---

## 2. 全局注解（main.go 顶部）

```go
// @title           Spring Slumber API
// @version         1.0
// @description     后端 REST 接口文档
// @BasePath        /
// @schemes         http https

// @securityDefinitions.apikey  BearerAuth
// @in                          header
// @name                        Authorization
// @description                 形如 `Bearer <jwt>`
package main
```

字段对照 OpenAPI 3 顶层（实际生成 OpenAPI 2.0/Swagger 2.0）。

---

## 3. Handler 注解模板

```go
// Login 用户登录（手机号 + 短信验证码）。
// @Summary  用户登录
// @Description 校验手机号和短信码，成功后返回 JWT。
// @Tags     user
// @Accept   json
// @Produce  json
// @Param    req  body      LoginRequest  true  "登录请求"
// @Success  200  {object}  response.Envelope{data=LoginResponse}
// @Failure  400  {object}  response.Envelope{error=response.Error}
// @Failure  401  {object}  response.Envelope{error=response.Error}
// @Router   /api/v1/user/login [post]
// @Security BearerAuth
func (h *Handler) Login(c *gin.Context) { ... }
```

### 3.1 注解速查

| 注解 | 作用 |
|---|---|
| `@Summary` | 短描述 |
| `@Description` | 详细描述 |
| `@Tags` | 分组（Swagger UI 标签） |
| `@Accept` / `@Produce` | 接受 / 返回 MIME，`json`、`form`、`xml`、`multipart` |
| `@Param` | 参数：`name in type required "desc"` |
| `@Success` / `@Failure` | 响应：`status {object} Type "desc"` |
| `@Router` | `path [method]` |
| `@Security` | 引用的 security 定义名（如 `BearerAuth`） |
| `@ID` | operationId |
| `@Deprecated` | 标弃用 |

### 3.2 `@Param` 的 in 取值

```text
query, header, path, cookie, body, formData
```

例：

```go
// @Param   id     path   int     true   "用户 ID"
// @Param   page   query  int     false  "页码"
// @Param   token  header string  false  "会话 token"
// @Param   user   body   LoginRequest  true "登录请求"
```

---

## 4. 类型注解

### 4.1 struct 上方

```go
// LoginRequest 登录请求体。
type LoginRequest struct {
    PhoneNumber string `json:"phoneNumber" binding:"required,e164" example:"13800000000"`
    Code        string `json:"code"        binding:"required,len=6" example:"123456"`
}
```

`example` tag 会带进 OpenAPI 示例。

### 4.2 通用 envelope 复用

```go
// 引用其它类型：response.Envelope{data=LoginResponse}
// 数组：[]LoginResponse 或 response.Envelope{data=[]LoginResponse}
type LoginResponse struct {
    AccessToken  string    `json:"accessToken"`
    RefreshToken string    `json:"refreshToken"`
    ExpiresAt    time.Time `json:"expiresAt"`
}
```

### 4.3 嵌套引用

```go
// @Success 200 {object} PageResponse{data=User,meta=PageMeta}
type PageResponse struct {
    Data any `json:"data"`
    Meta any `json:"meta"`
}
```

---

## 5. 集成 Gin

### 5.1 路由

```go
import (
    httpSwagger "github.com/swaggo/http-swagger"
    _ "your-project/internal/docs"  // 匿名导入注册 doc.json
)

r.GET("/swagger/*any", gin.WrapH(httpSwagger.Handler(
    httpSwagger.URL("/swagger/doc.json"),
)))
```

`/swagger/doc.json` 即 OpenAPI JSON；`/swagger/index.html` Swagger UI。

### 5.2 多套文档（v1 / v2）

```go
r.GET("/swagger/v1/*any", gin.WrapH(httpSwagger.Handler(
    httpSwagger.URL("/swagger/v1/doc.json"),
)))
r.GET("/swagger/v2/*any", gin.WrapH(httpSwagger.Handler(
    httpSwagger.URL("/swagger/v2/doc.json"),
)))
```

### 5.3 dev/prod 开关

```go
if cfg.App.Env != "production" {
    r.GET("/swagger/*any", gin.WrapH(httpSwagger.Handler(...)))
}
```

生产环境通常用 BasicAuth 或仅内网开放。

---

## 6. swag generate 流程

```makefile
# Makefile
swag:
    swag init -g cmd/server/main.go -o internal/docs --parseDependency --parseInternal

.PHONY: swag
```

CI 钩子：

```yaml
# .github/workflows/ci.yml
- name: Generate OpenAPI
  run: go run github.com/swaggo/swag/cmd/swag init -g cmd/server/main.go -o internal/docs --parseDependency
- name: Check diff
  run: git diff --exit-code internal/docs
```

确保每次提交都包含生成的文档。

---

## 7. 进阶

### 7.1 多文件 main

`swag init` 支持 `-g` 指定多入口；同 package 内多个 main 文件需 `//go:build ignore` 隔离。

### 7.2 自定义扩展字段

```go
// @x-codeSamples
// @x-extension-field  value
```

`x-` 前缀直接透传 OpenAPI 字段。

### 7.3 多个 security scheme

```go
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization

// @securityDefinitions.oauth2.application OAuth2
// @tokenUrl https://example.com/oauth/token
// @scope.read Grants read access
// @scope.write Grants write access
```

---

## 8. 与项目代码风格一致的做法

### 8.1 信封统一

本项目所有 handler 通过 `response.JSON / response.Problem` 输出。建议在 swag 注解里也明确写出信封结构，避免前端误读：

```go
// @Success 200 {object} response.Envelope{data=LoginResponse}
// @Failure 401 {object} response.Envelope{error=response.Error}
```

`response.Envelope` 与 `response.Error` 都是公开类型，可直接引用。

### 8.2 把 request struct 放在 handler 同包

避免 swag 跨包找不到类型。

### 8.3 internal 包路径

`internal/docs` 用 `_ "..."` 匿名导入，注册 init() 把 doc.json 注入 swag 运行时。

---

## 9. 故障排查

| 现象 | 原因 | 处理 |
|---|---|---|
| swagger UI 显示「Failed to load spec」 | 没导入 `internal/docs` | 加 `_ "internal/docs"` |
| 找不到 type | 类型未导出 / 在外部包未导出 | 用导出名字，swag 解析导出符号 |
| `ParseDependency` 找不到其它包结构 | 缺 `--parseDependency` | 加 flag |
| 返回字段为空 | struct 没 `json` tag | 加 tag |
| example 不生效 | 没 `example` tag | 加 `example:"xxx"` |
| body schema 把 json 字段全展平了 | type 用了 `interface{}` | 给具体类型 |

---

## 10. 官方推荐 vs 反模式

| ❌ | ✅ |
|---|---|
| 手写 doc.json | 用 swag 从注解生成 |
| 注解里写大段英文描述 | 简洁一句话，关键信息外置到 wiki |
| 不写 `@Router`，路由与文档不同步 | 注解 = 单一真源 |
| 公开敏感端点到生产 | 走白名单 / 内网 |
| 不带 `@Security` | 标注每个受保护接口的安全定义 |
| response 写 `interface{}` | 用具体 struct |

---

## 11. 替代方案

| 工具 | 特点 |
|---|---|
| [swaggo/swag](https://github.com/swaggo/swag) | 注解式，本项目使用 |
| [oapi-codegen](https://github.com/oapi-codegen/oapi-codegen) | 先写 OpenAPI，再生成代码 |
| [kin-openapi](https://github.com/getkin/kin-openapi) | 运行时校验 |
| [ogen](https://github.com/ogen-go/ogen) | 先 OpenAPI，零反射生成 |

---

## 相关资料链接

- [swaggo/swag GitHub](https://github.com/swaggo/swag)
- [swaggo/http-swagger GitHub](https://github.com/swaggo/http-swagger)
- [Swagger 2.0 Spec](https://swagger.io/specification/v2/)
- [OpenAPI 3.0 Spec](https://swagger.io/specification/)
- [swag 示例项目](https://github.com/swaggo/swag/tree/master/example)
