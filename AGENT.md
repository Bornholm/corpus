# AGENT.md

This file provides guidance for AI coding agents working on the Corpus codebase.

## Project Overview

Corpus is a RAG (Retrieval-Augmented Generation) service written in Go. It provides document indexing, full-text and vector search, LLM-based result ranking, and a web interface — all designed to be self-hostable and easy to deploy.

## Repository Structure

```
.
├── cmd/
│   ├── desktop/          # Desktop application entry point (self-update, tray)
│   └── server/           # Server CLI entry point
│       ├── common/       # Shared flags and resolvers
│       ├── index/        # Index subcommand
│       └── watch/        # Watch subcommand (filesystem auto-indexing)
├── config/               # Environment-based configuration structs
├── internal/
│   ├── adapter/          # External system adapters (bleve, sqlite-vec, etc.)
│   ├── core/
│   │   ├── model/        # Domain model (entities, value objects, interfaces)
│   │   ├── port/         # Interfaces (ports) and test suites
│   │   └── service/      # Business logic (document manager, backup, etc.)
│   ├── crypto/           # Cryptographic utilities
│   ├── desktop/          # Desktop UI (templ components, handlers)
│   ├── fileconverter/    # File format conversion (routing, retry, rate limiting)
│   ├── filesystem/       # Abstract filesystem backends (S3, FTP, SFTP, SMB, WebDAV)
│   ├── http/
│   │   ├── context/      # HTTP request context helpers
│   │   ├── handler/      # HTTP handlers (REST API, MCP, WebUI)
│   │   │   ├── mcp/      # Model Context Protocol handler
│   │   │   └── webui/    # Templ-based web interface handlers and components
│   │   ├── middleware/   # HTTP middleware (authz, rate limiting, etc.)
│   │   └── url/          # URL mutation helpers
│   ├── llm/              # LLM client wrappers (logging, retries)
│   └── scraper/          # HTTP and browser-based content scrapers
├── misc/                 # Deployment helpers (dokku, docker, etc.)
└── go.mod
```

## Architecture

Corpus follows a **hexagonal (ports and adapters) architecture**:

- **`internal/core/model/`** — Pure domain entities. No external dependencies. All domain types are defined as interfaces where appropriate, with concrete `Base*` implementations.
- **`internal/core/port/`** — Interfaces that define what the core needs from the outside world (storage, indexing, file conversion, task running). Adapters implement these interfaces.
- **`internal/core/service/`** — Business logic that orchestrates domain objects through ports. No direct dependency on HTTP, databases, or external services.
- **`internal/adapter/`** — Concrete implementations of ports (e.g., Bleve for full-text search, SQLite Vec for vector search).
- **`internal/http/`** — HTTP delivery layer. Handlers translate HTTP requests into service calls and render responses.

## Code Conventions

### Go Style

- **Go version**: 1.23+ (see `go.mod` and `.github/workflows/release.yml`).
- Follow standard [Effective Go](https://go.dev/doc/effective_go) guidelines.
- Use `github.com/pkg/errors` for error wrapping: `errors.WithStack(err)`, `errors.Wrapf(err, "context: %v", val)`. Never use bare `fmt.Errorf` with `%w` in this codebase.
- Prefer `errors.WithStack` when re-returning an error without adding context. Use `errors.Wrapf` when adding context.
- All exported types that implement an interface must include a compile-time assertion at the bottom of the file:
  ```go
  var _ port.FileConverter = &RoutedFileConverter{}
  ```
- Use `log/slog` for structured logging. Use `slog.DebugContext`, `slog.InfoContext`, `slog.ErrorContext` — always pass the context.
- Enrich log contexts with `slogx.WithAttrs(ctx, slog.String("key", "value"))` from `github.com/bornholm/go-x/slogx`.

### Domain Model (`internal/core/model/`)

- Domain entities are **interfaces** (`Document`, `Collection`, `User`, `PublicShare`, etc.).
- Concrete implementations are named `Base*` (e.g., `BaseCollectionShare`) and are unexported or package-private where possible.
- IDs are typed strings (e.g., `type CollectionID string`) with a constructor using `xid`: `func NewCollectionID() CollectionID { return CollectionID(xid.New().String()) }`.
- Lifecycle mixins (`WithLifecycle`, `WithOwner`, `WithID`) are composed via embedding.
- New model files follow this pattern:

  ```go
  package model

  import "github.com/rs/xid"

  type FooID string

  func NewFooID() FooID {
      return FooID(xid.New().String())
  }

  type Foo interface {
      WithID[FooID]
      // ... domain methods
  }
  ```

### Ports (`internal/core/port/`)

- Define interfaces for every external dependency the core needs.
- Errors specific to a port are defined in `port/error.go` (e.g., `port.ErrNotSupported`).
- Test suites for port implementations live in `port/testsuite/`.

### HTTP Handlers (`internal/http/handler/`)

- Each handler group is a struct with a `ServeHTTP` method implementing `http.Handler`.
- Routing uses the standard `net/http` `ServeMux` (Go 1.22+ pattern-based routing).
- Context values are managed through `internal/http/context/` helpers — do not use raw `context.WithValue` in handlers; add typed accessors there.
- Authentication and authorization are handled via middleware in `internal/http/middleware/`.
- Rate limiting headers follow the pattern: `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`.

### Web UI (`internal/http/handler/webui/`)

- Templates use [templ](https://templ.guide/) (`.templ` files). Do not edit generated `*_templ.go` files directly.
- UI uses [HTMX](https://htmx.org/). Do not introduce other CSS frameworks.
- Markdown rendering uses `github.com/yuin/goldmark` via the `common.Markdown(source)` templ component.
- Use `common.BaseURL(ctx, common.WithPath(...))` to construct links in components or redirects
- Use `make generate` to generate templ components

### Configuration (`config/`)

- Configuration is loaded from environment variables using `github.com/caarlos0/env/v11`.
- Each subsystem has its own config file (`http.go`, `llm.go`, `storage.go`, etc.).

### File Converters (`internal/fileconverter/`)

- Implement `port.FileConverter` (has `Convert(ctx, filename, r io.Reader) (io.ReadCloser, error)` and `SupportedExtensions() []string`).
- Use `RoutedFileConverter` to compose multiple converters by extension.
- Wrap converters with `RetryFileConverter` and rate-limited variants where appropriate.

### Filesystem Backends (`internal/filesystem/backend/`)

- Each backend (FTP, SFTP, S3, SMB, WebDAV, local) implements the abstract filesystem interface.
- Connection options are parsed from DSN-style URLs with query parameters.

## Testing

- Use table-driven tests where multiple cases apply.
- Port implementations must pass the shared test suite in `internal/core/port/testsuite/`.
- Use `testdata/` directories for fixture files (e.g., `.md` documents for indexing tests).
- Test files are named `*_test.go` and live alongside the code they test.

## Error Handling

- **Always** wrap errors before returning: `return errors.WithStack(err)` or `return errors.Wrapf(err, "doing X with %v", param)`.
- Do **not** silently swallow errors. Log them at the appropriate level if they cannot be propagated.
- Domain-level "not found" and "not supported" sentinel errors live in `internal/core/port/error.go`.

## Dependencies

Key dependencies and their roles:

| Dependency                                 | Role                             |
| ------------------------------------------ | -------------------------------- |
| `github.com/blevesearch/bleve/v2`          | Full-text search index           |
| `github.com/asg017/sqlite-vec-go-bindings` | Vector search index              |
| `github.com/ncruces/go-sqlite3`            | SQLite driver                    |
| `github.com/bornholm/genai`                | LLM client abstraction           |
| `github.com/a-h/templ`                     | Type-safe HTML templating        |
| `github.com/mark3labs/mcp-go`              | Model Context Protocol server    |
| `github.com/pkg/errors`                    | Error wrapping with stack traces |
| `github.com/rs/xid`                        | Globally unique IDs              |
| `github.com/markbates/goth`                | OIDC / OAuth2 authentication     |
| `github.com/minio/minio-go/v7`             | S3-compatible storage            |
| `github.com/gorilla/sessions`              | HTTP session management          |
| `github.com/caarlos0/env/v11`              | Environment-based config         |

## Build & Release

- Build is managed by [GoReleaser](https://goreleaser.com/) (see `.goreleaser.yml`).
- CI runs on GitHub Actions (`.github/workflows/release.yml`).
- Docker images are published to `ghcr.io`.
- The desktop binary supports self-update via `github.com/creativeprojects/go-selfupdate` with checksum validation.

## What NOT to Do

- Do not add direct database calls or HTTP calls inside `internal/core/model/` or `internal/core/service/`. Use ports.
- Do not use `fmt.Errorf` with `%w` — use `github.com/pkg/errors` consistently.
- Do not edit generated `*_templ.go` files.
- Do not introduce new CSS frameworks or JS bundlers. Stick to Bulma + HTMX + vanilla JS.
- Do not store secrets or credentials in code. Use environment variables via the `config/` layer.
- Do not bypass the `port` interfaces from handlers — always go through `service` or `port` abstractions.
