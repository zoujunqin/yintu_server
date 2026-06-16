# Spring Slumber Server

Go HTTP server providing REST API endpoints for the Spring Slumber application.

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