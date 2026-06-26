# Spring Slumber Server

* 每次编写代码时，需要**一层一层的分析代码关联**之后，再分析需求，然后对照需求哪个改动是最少代码最完美实现，减少幻觉。

## Tech Stack

- **Language:** Go 1.22+
- **HTTP Framework:** [Gin](https://github.com/gin-gonic/gin) (`gin.Engine` + `gin.RouterGroup`)
- **Config:** Environment variables with `.env` file support
- **Architecture:** Clean layered: handler → service → response

## Commands

```bash
go run ./cmd/server/    # Run the server
go build ./cmd/server/  # Build the binary
go test ./...           # Run all tests
```

Or use the PowerShell dev script:
```powershell
./scripts/dev.ps1
```

## Architecture

```
cmd/server/main.go          # Entry point: config load, signal handling, graceful shutdown
internal/
├── config/
│   ├── config.go           # AppConfig, HTTPConfig, CORSConfig + Load()
│   └── dotenv.go           # .env parser
├── handler/
│   ├── health.go           # GET /, /healthz, /readyz
│   └── overview.go         # GET /api/v1/overview
├── httpserver/
│   ├── server.go           # *http.Server wrapper around gin.Engine; Start/Shutdown
│   ├── router.go           # gin.Engine + middleware chain
│   ├── group.go            # RouterGroup wrapping gin.RouterGroup, impls feature.Router
│   └── middleware.go       # RequestID, RequestLogger, Recoverer, CORS (gin.HandlerFunc)
└── response/
    └── response.go         # JSON envelope helpers (Gin-flavored)
```

## Coding Conventions

- **Package naming:** lowercase, no underscores, no camelCase
- **File naming:** `snake_case.go` for all Go files
- **Imports:** stdlib first, then third-party, then internal packages
- **Error handling:** Always check and handle errors, propagate with context
- **Config:** All configuration via environment variables, use `config.Load()` to parse
- **HTTP handlers:** Implement `func(c *gin.Context)`, respond with `response.JSON` / `response.Problem` / `response.NoContent` for the unified envelope
- **Middleware:** Implement `gin.HandlerFunc`; register globally via `engine.Use(...)` or per-group via `NewRouterGroup(engine, prefix, mw...)`
- **Route registration:** Features implement `feature.Router` (wraps `gin.RouterGroup`); handlers receive `gin.HandlerFunc` and don't see prefix strings
- **Graceful shutdown:** Catch SIGINT/SIGTERM, call `server.Shutdown()` with context timeout
- **Logging:** Use structured logging with request ID in context

## API Response Format

All API responses use this JSON envelope:
```json
{
  "data": { ... },
  "error": { "code": "...", "message": "..." }
}
```

For errors, use `response.Problem(c, statusCode, code, message)`.
For success, use `response.JSON(c, statusCode, payload)`.
For no-content, use `response.NoContent(c)`.

## Environment Variables

Copy `.env.example` to `.env` and configure. Key variables:
- `HTTP_LISTEN` — server address (default: `0.0.0.0:8080`)
- `CORS_ORIGINS` — allowed origins (comma-separated)
- `APP_ENV` — `development` | `production`

## Reference Documentation

Before touching any code in this repo, **consult the official API references** in `.claude/notes/` — they are the single source of truth for *how* each library is supposed to be used, sourced only from official docs (go.dev / pkg.go.dev / gin-gonic.com / gorm.io / official GitHub repos):

| File | Topic | Read when… |
|---|---|---|
| `notes/go-stdlib.md` | Go 1.22+ stdlib (`net/http`, `context`, `log/slog`, `errors`, `sync/errgroup`) | working with servers, goroutines, logging, errors |
| `notes/gin.md` | Gin v1.10+ routing, middleware, binding, validation | adding handlers, routes, middleware |
| `notes/gorm-postgres.md` | GORM v2 + PostgreSQL (models, queries, transactions, hooks, error codes) | writing models / DAOs / migrations |
| `notes/jwt.md` | golang-jwt/jwt v5 (issue / parse / refresh / revoke) | touching `internal/auth/*` |
| `notes/swag-openapi.md` | swag annotations → OpenAPI doc.json | adding new API endpoints or running `make swag` |
| `notes/README.md` | index + maintenance conventions | first read in any new session |

Rules of engagement with these notes:
- ✅ Treat them as canonical — match project code to the patterns they describe, not the other way around.
- ✅ When proposing a code change that uses an API listed in a note, cite the relevant section.
- ❌ Do not invent patterns not covered by the notes — if you need something new, either extend the note (with an official source) or ask first.