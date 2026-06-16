# spring-slumber-server

Go backend template for Spring Slumber.

## Structure

```text
cmd/server              application entrypoint
internal/config         environment-based configuration
internal/handler        HTTP handlers
internal/httpserver     server, router, middleware
internal/response       response helpers
scripts                 local development scripts
```

## Requirements

- Go 1.22 or newer

## Run

```powershell
Copy-Item .env.example .env
go run ./cmd/server
```

Default server address: `http://localhost:8080`.

## Endpoints

- `GET /healthz` / `GET /readyz` — 健康检查
- `GET /api/v1/overview` — 服务概览
- `POST /api/v1/user/send-code` — 发送短信验证码
- `POST /api/v1/user/login` — 手机号 + 验证码登录，返回 JWT

## API 文档（Swagger UI）

服务启动后可直接访问：

- Swagger UI：<http://localhost:8080/swagger/index.html>
- OpenAPI JSON：<http://localhost:8080/swagger/doc.json>
- 别名：`/docs/index.html` 与 `/docs/doc.json`

文档基于 handler 上的 [swag](https://github.com/swaggo/swag) 注解自动生成；修改注解后重新生成：

```bash
# 在 apps/spring-slumber-server 目录下
make swag               # 需要本地 swag CLI（首次执行 make install-tools）
# 或者从 monorepo 根目录：
npm run docs:gen
```

## Useful Commands

```bash
# 在 apps/spring-slumber-server 目录下
make help            # 查看所有 Makefile 目标
make install-tools   # 安装 swag CLI
make swag            # 生成 Swagger 文档
make build           # 编译二进制到 bin/server
make run             # 本地启动（默认 :8080）
make test            # 跑单测
make tidy            # go mod tidy

# 等价的 Go 原生命令
go fmt ./...
go test ./...
go run ./cmd/server
```
