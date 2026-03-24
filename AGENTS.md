# AGENTS.md

This file provides guidance to LLM agents when working with code in this repository.

## Project Overview

Corpus is a self-hostable RAG (Retrieval-Augmented Generation) service in Go. It indexes documents (Markdown, Office, PDF via converters), provides full-text search (Bleve) and vector search (SQLite Vec), ranks results with LLM assistance, and serves a web UI + REST API + MCP server. It can also be used as an embeddable Go library via `pkg/corpus`.

## Commands

```bash
# Development
make watch          # Auto-rebuild and run server on file changes (uses modd + .env)
make generate       # Regenerate templ templates + Tailwind CSS (required after editing .templ files)
make build          # Build all binaries (server, client, desktop) to ./bin/
make build-server   # Build server binary only
make purge          # Remove SQLite databases and Bleve index

# Testing
go test ./...                                  # Run all tests (CGO required for sqlitevec)
go test ./pkg/adapter/bleve/...               # Run tests for a specific package
go test -v -run TestName ./pkg/...            # Run a single test by name

# Running with env vars
make CMD='bin/server' run-with-env            # Run a binary loading .env
cp .env.dist .env                             # Initial setup
```

> **Note:** `pkg/adapter/sqlitevec` uses CGO. Ensure a C compiler is available.

## Architecture

Corpus follows **hexagonal (ports and adapters) architecture**.

### Package layout

- **`pkg/model/`** — Pure domain interfaces (`Document`, `Section`, `Collection`, `User`, `Task`, …). IDs are typed strings using xid.
- **`pkg/port/`** — Port interfaces (`DocumentStore`, `Index`, `UserStore`, `TaskRunner`, `FileConverter`). Sentinel errors in `port/error.go`. Shared test suites in `port/testsuite/`.
- **`pkg/adapter/`** — Port implementations:
  - `bleve/` — Full-text search
  - `sqlitevec/` — Vector search (HNSW via sqlite-vec, CGO)
  - `pipeline/` — Composite index combining weighted sub-indexes with `QueryTransformer` and `ResultsTransformer` chains (HyDE, deduplication, LLM judge)
  - `gorm/` — SQLite persistence via GORM (documents, sections, collections, users)
  - `cache/` — In-memory LRU cache wrappers for stores
  - `memory/` — In-memory task runner
  - `genai/` — LLM-based file converter (via `github.com/bornholm/genai`)
  - `pandoc/`, `libreoffice/` — External file converters
- **`pkg/corpus/`** — Embeddable high-level API (`corpus.New()`, auto-composition of all adapters).
- **`internal/core/service/`** — Business logic: `DocumentManager` (search/indexing/ranking/ask), `backup`.
- **`internal/setup/`** — Server-side wiring: reads env config, instantiates all components, registers task handlers.
- **`internal/http/`** — HTTP delivery: `handler/api/` (REST), `handler/mcp/` (Model Context Protocol), `handler/webui/` (templ + HTMX UI), `middleware/` (authn, authz, rate limiting).
- **`internal/task/document/`** — Async task handlers: `IndexFileHandler`, `CleanupHandler`, `ReindexHandler`.
- **`internal/markdown/`** — Markdown parser that splits documents into hierarchical sections.
- **`internal/fileconverter/`** — `RoutedFileConverter` dispatches by extension; wraps converters with retry and rate limiting.
- **`internal/filesystem/`** — Abstract FS backends: local, S3, FTP, SFTP, SMB, WebDAV, Git.
- **`internal/config/`** — Environment-based config structs via `github.com/caarlos0/env/v11`.
- **`cmd/server/`**, **`cmd/client/`**, **`cmd/desktop/`** — Binary entry points.

### Migration in progress

`pkg/model/`, `pkg/port/`, and `pkg/adapter/` are the result of an ongoing migration of `internal/core/model/`, `internal/core/port/`, and `internal/adapter/`. The `internal/` packages (http, setup, task, etc.) import from `pkg/` now. Do not create new types in the old `internal/core/` paths.

### Search pipeline

1. `HyDEQueryTransformer` — LLM generates a hypothetical answer to expand the query.
2. Parallel search across weighted indexes (Bleve 0.4 + SQLiteVec 0.6 by default).
3. `mergeResults` — score fusion weighted by index weight.
4. `DuplicateContentResultsTransformer` — keeps only leaf sections (removes parent when child is present).
5. `JudgeResultsTransformer` — LLM judges relevance, returns only relevant section IDs.

### Embeddable library (`pkg/corpus`)

```go
c, err := corpus.New(ctx,
    corpus.WithStoragePath("./data"),
    corpus.WithLLMClient(llmClient),
    // corpus.WithDisableHyDE(),
    // corpus.WithDisableJudge(),
)
collID, _ := c.CreateCollection(ctx, "docs")
taskID, _ := c.IndexFile(ctx, collID, "doc.md", reader)
results, _ := c.Search(ctx, "query", corpus.WithSearchCollections(collID))
answer, _, _ := c.Ask(ctx, "question", results)
```

Low-level: pass explicit components via `corpus.WithIndex(idx)`, `corpus.WithDocumentStore(store)`, etc.

## Code Conventions

### Error handling

- **Always** use `github.com/pkg/errors`: `errors.WithStack(err)` or `errors.Wrapf(err, "context: %v", val)`.
- **Never** use `fmt.Errorf` with `%w`.
- Domain sentinel errors live in `pkg/port/error.go`.

### Go style

- Use `log/slog` with context: `slog.DebugContext`, `slog.InfoContext`, `slog.ErrorContext`.
- Enrich log contexts via `slogx.WithAttrs(ctx, slog.String("key", "value"))` from `github.com/bornholm/go-x/slogx`.
- All exported types implementing an interface must have a compile-time assertion: `var _ port.Index = &Index{}`.

### Domain model

- Entities are interfaces; concrete types implement them.
- IDs are typed strings: `type FooID string` with `func NewFooID() FooID { return FooID(xid.New().String()) }`.

### HTTP handlers

- Each handler group is a struct implementing `http.Handler` via `ServeHTTP`.
- Routing uses standard `net/http.ServeMux` (Go 1.22+ patterns).
- Use `internal/http/context/` typed helpers — never `context.WithValue` directly in handlers.

### Web UI

- Templates are `.templ` files — **never edit generated `*_templ.go` files** directly. Run `make generate` after editing `.templ` files.
- UI stack: Bulma CSS + HTMX + vanilla JS.

## What NOT to Do

- No direct DB/HTTP calls inside `pkg/model/` or `internal/core/service/` — use port interfaces.
- No `fmt.Errorf` with `%w` — use `github.com/pkg/errors`.
- Do not edit `*_templ.go` generated files.
- Do not bypass port interfaces from handlers — always go through service/port abstractions.
- Do not create new types in `internal/core/model/` or `internal/core/port/` — use `pkg/model/` and `pkg/port/`.
