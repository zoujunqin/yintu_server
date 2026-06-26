# .claude/notes 索引

> 本目录沉淀本项目相关 Go 生态官方文档知识，**仅供本项目开发参考**。
> 所有内容只来自官方文档（go.dev、pkg.go.dev、gin-gonic.com、gorm.io、github.com 各仓库 README/wiki）。
> 编写日期：2026-06-19。

## 项目当前技术栈

| 组件 | 版本 | 笔记 |
|---|---|---|
| Go | 1.25.0 | [go-stdlib](./go-stdlib.md) |
| Gin | v1.12.0 | [gin](./gin.md) |
| GORM | v1.25.12 | [gorm-postgres](./gorm-postgres.md) |
| Postgres Driver | v1.5.9 | [gorm-postgres](./gorm-postgres.md) |
| JWT | v5.3.1 | [jwt](./jwt.md) |
| swag | v1.16.6 | [swag-openapi](./swag-openapi.md) |
| http-swagger | v1.3.4 | [swag-openapi](./swag-openapi.md) |

## 笔记维护约定

- ✅ 每份笔记顶部 frontmatter 含 `name` / `description` / `type: reference`。
- ✅ 顶部统一标注官方来源 URL。
- ✅ 内容仅写「官方怎么说的」+ 「本项目怎么用的」。
- ❌ 不收录第三方博客、教程、stackoverflow。
- ❌ 不收录已不再维护的版本（jwt v4、gin v1.7 之前的 API）。

## 与 .claude/rules/go-dev.md 的关系

- `rules/go-dev.md`：项目**强制**规范（如包命名、错误处理、接口设计），每次写代码必须遵守。
- `notes/*.md`：对应规范的**官方 API 速查**，按需查阅。

新加入成员建议顺序：先读 `rules/go-dev.md` → 再按任务读相关 notes。

## 笔记索引

| 笔记 | 一句话定位 |
|---|---|
| [go-stdlib](./go-stdlib.md) | net/http + context + slog + errors + errgroup + testing |
| [gin](./gin.md) | 路由、中间件、绑定、验证、统一错误信封 |
| [gorm-postgres](./gorm-postgres.md) | 连接池、模型、查询、事务、Hook、错误码 |
| [jwt](./jwt.md) | 签发、解析、刷新、撤销、Gin 中间件 |
| [swag-openapi](./swag-openapi.md) | 注解 → doc.json → Swagger UI |

## 下次维护 TODO

- [ ] 补 `gin-contrib/*` 中间件实践笔记（cors/requestid/timeout）
- [ ] 补 `golang-migrate` 或 `pressly/goose` 数据库迁移笔记
- [ ] 补 `testcontainers-go` 集成测试笔记
- [ ] 补 `slog` 进阶：自定义 Handler、OpenTelemetry bridge
- [ ] 跟进 Gin v1.13+ 新 API（如有）
