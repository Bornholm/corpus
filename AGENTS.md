# AGENTS.md

This file provides guidance to the LLM agents when working with code in this repository.

## Project Overview

Corpus is a self-hostable RAG (Retrieval-Augmented Generation) service in Go. It indexes documents (Markdown, Office, PDF via converters), provides full-text search (Bleve) and vector search (SQLite Vec), ranks results with LLM assistance, and serves a web UI + REST API + MCP server.

## Commands

```bash
# Development
make watch          # Auto-rebuild and run server on file changes (uses modd + .env)
make generate       # Regenerate templ templates + Tailwind CSS (required after editing .templ files)
make build          # Build all binaries (server, client, desktop) to ./bin/
make build-server   # Build server binary only
make purge          # Remove SQLite databases and Bleve index

# Testing
go test ./...                          # Run all tests
go test ./internal/adapter/bleve/...  # Run tests for a specific package
go test -v -run TestName ./pkg/...    # Run a single test by name
```

## Architecture

Corpus follows **hexagonal (ports and adapters) architecture**:

- **`internal/core/model/`** — Pure domain entities. Interfaces + `Base*` concrete implementations. No external dependencies.
- **`internal/core/port/`** — Interfaces for external dependencies (storage, indexing, file conversion, task running). Sentinel errors in `port/error.go`. Shared test suites in `port/testsuite/`.
- **`internal/core/service/`** — Business logic (`DocumentManager` for search/indexing/ranking, `backup`). No direct HTTP/DB dependencies.
- **`internal/adapter/`** — Port implementations: `bleve/` (full-text), `sqlitevec/` (vector), `gorm/` (SQLite via GORM), `genai/` (LLM), `pandoc/`, `libreoffice/`, `cache/`, `pipeline/`.
- **`internal/http/`** — HTTP delivery: `handler/api/` (REST), `handler/mcp/` (Model Context Protocol), `handler/webui/` (templ + HTMX UI), `middleware/` (authn, rate limiting), `context/` (typed context helpers).
- **`internal/filesystem/backend/`** — Filesystem adapters: local, S3, FTP, SFTP, SMB, WebDAV, Git.
- **`internal/fileconverter/`** — `RoutedFileConverter` dispatches by extension; wraps converters with retry and rate limiting.
- **`config/`** — Environment-based config structs (via `github.com/caarlos0/env/v11`), one file per subsystem.

## Code Conventions

### Error Handling

- **Always** use `github.com/pkg/errors`: `errors.WithStack(err)` or `errors.Wrapf(err, "context: %v", val)`.
- **Never** use `fmt.Errorf` with `%w` — this codebase uses `pkg/errors` exclusively.
- Domain-level sentinel errors live in `internal/core/port/error.go`.

### Go Style

- Use `log/slog` with context: `slog.DebugContext`, `slog.InfoContext`, `slog.ErrorContext`.
- Enrich log contexts via `slogx.WithAttrs(ctx, slog.String("key", "value"))` from `github.com/bornholm/go-x/slogx`.
- All exported types implementing an interface must have a compile-time assertion:
  ```go
  var _ port.FileConverter = &RoutedFileConverter{}
  ```

### Domain Model (`internal/core/model/`)

- Entities are interfaces; concrete types are `Base*`.
- IDs are typed strings: `type FooID string` with `func NewFooID() FooID { return FooID(xid.New().String()) }`.
- Lifecycle mixins (`WithLifecycle`, `WithOwner`, `WithID`) are composed via embedding.

### HTTP Handlers

- Each handler group is a struct implementing `http.Handler` via `ServeHTTP`.
- Routing uses standard `net/http.ServeMux` (Go 1.22+ patterns).
- Use `internal/http/context/` typed helpers — never `context.WithValue` directly in handlers.
- Construct URLs with `common.BaseURL(ctx, common.WithPath(...))`.

### Web UI

- Templates are `.templ` files — **never edit generated `*_templ.go` files**.
- UI stack: Bulma CSS + HTMX + vanilla JS. Do not introduce other frameworks.
- Markdown rendering: `common.Markdown(source)` templ component (goldmark).

### Testing

- Use table-driven tests where multiple cases apply.
- Port implementations must pass the shared test suite in `internal/core/port/testsuite/`.
- Use `testdata/` directories for fixture files.

## What NOT to Do

- No direct DB/HTTP calls inside `internal/core/model/` or `internal/core/service/` — use ports.
- No `fmt.Errorf` with `%w` — use `github.com/pkg/errors`.
- Do not edit `*_templ.go` generated files.
- Do not bypass port interfaces from handlers — always go through service/port abstractions.
- Do not introduce new CSS frameworks or JS bundlers.
