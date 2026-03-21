<p align="center">
  <img src="https://raw.githubusercontent.com/Bornholm/corpus/refs/heads/main/internal/http/handler/webui/common/assets/logo.svg" width="128px" alt="Logo"/>
</p>

# Corpus

An easy to deploy [RAG](https://en.wikipedia.org/wiki/Retrieval-augmented_generation) service.

> ⚠️ **Disclaimer**
>
> Corpus is currently under active development and should be considered a work in progress. The API is in a preliminary stage and may not be stable. Please be aware that changes, including modifications, updates, or deprecations, can occur at any time without prior notice.

## Features

- OIDC authentication with email-based role mapping and access whitelist
- Markdown-based chunking
- Use full-text and vector-based indexes (via [Bleve](https://github.com/blevesearch/bleve) and [SQLite Vec](https://github.com/asg017/sqlite-vec-go-bindings))
- Web interface and REST API
- Backup and restore via the REST API
- CLI with abstract filesystem watching and auto-indexing (local, S3, FTP, SFTP, WebDAV, SMB...)

## Getting started

### With Docker

```bash
# Create data volume
docker volume create corpus_data

# Start container
docker run \
  -it --rm \
  -v corpus_data:/data \
  --net=host \
  -e CORPUS_LLM_PROVIDER_KEY="<LLM_SERVICE_API_KEY>" \
  -e CORPUS_LLM_PROVIDER_BASE_URL="<LLM_SERVICE_BASE_URL>" \
  -e CORPUS_LLM_PROVIDER_CHAT_COMPLETION_MODEL="<LLM_SERVICE_CHAT_COMPLETION_MODEL>" \
  -e CORPUS_LLM_PROVIDER_EMBEDDINGS_MODEL="<LLM_SERVICE_EMBEDDINGS_MODEL>" \
  -e CORPUS_HTTP_AUTHN_PROVIDERS_GITHUB_KEY="<github_oauth2_app_key>"
  -e CORPUS_HTTP_AUTHN_PROVIDERS_GITHUB_SECRET="<github_oauth2_app_secret>" \
  -e CORPUS_HTTP_AUTHN_PROVIDERS_GITHUB_SCOPES=openid,user \
  ghcr.io/bornholm/corpus-server:latest
```

Then open http://localhost:3002 in your browser.

**Examples**

With a local [ollama](https://ollama.com/) instance:

```bash
CORPUS_LLM_PROVIDER_BASE_URL=http://127.0.0.1:11434/v1/
CORPUS_LLM_PROVIDER_CHAT_COMPLETION_MODEL=qwen2.5:7b
CORPUS_LLM_PROVIDER_EMBEDDINGS_MODEL=mxbai-embed-large
```

With [Mistral](https://mistral.ai/):

```bash
CORPUS_LLM_PROVIDER_BASE_URL=https://api.mistral.ai/v1/
CORPUS_LLM_PROVIDER_CHAT_COMPLETION_MODEL=mistral-small-latest
CORPUS_LLM_PROVIDER_EMBEDDINGS_MODEL=mistral-embed
```

## Usage as a library

Corpus can be embedded directly in a Go project to index and search documents without running a server.

### Installation

```bash
go get github.com/bornholm/corpus
```

> **Note:** The `sqlitevec` adapter uses CGO. Make sure a C compiler is available in your build environment.

### Quick start

```go
import (
    "context"

    "github.com/bornholm/corpus/pkg/corpus"
    "github.com/bornholm/genai/llm/provider"
    providerenv "github.com/bornholm/genai/llm/provider/env"
)

// Create a client from environment variables (see configuration below)
llmClient, _ := provider.Create(ctx, providerenv.With("LLM_", ".env"))

// Initialise an embedded corpus (SQLite + Bleve + SQLiteVec, auto-composed)
c, err := corpus.New(ctx,
    corpus.WithStoragePath("./data"),
    corpus.WithLLMClient(llmClient),
)

// Create a collection, index a file, search, ask
collID, _ := c.CreateCollection(ctx, "my-docs")
taskID, _ := c.IndexFile(ctx, collID, "notes.md", reader)

results, _ := c.Search(ctx, "my question",
    corpus.WithSearchCollections(collID),
    corpus.WithSearchMaxResults(5),
)

answer, _, _ := c.Ask(ctx, "my question", results)
```

### LLM configuration

The `providerenv.With(prefix)` helper reads LLM configuration from environment variables:

| Variable                              | Description              | Example (ollama)            |
| ------------------------------------- | ------------------------ | --------------------------- |
| `LLM_CHAT_COMPLETION_PROVIDER`        | Chat completion provider | `openai`                    |
| `LLM_CHAT_COMPLETION_OPENAI_BASE_URL` | Chat completion endpoint | `http://localhost:11434/v1` |
| `LLM_CHAT_COMPLETION_OPENAI_MODEL`    | Chat completion model    | `qwen2.5:7b`                |
| `LLM_EMBEDDINGS_PROVIDER`             | Embeddings provider      | `openai`                    |
| `LLM_EMBEDDINGS_OPENAI_BASE_URL`      | Embeddings endpoint      | `http://localhost:11434/v1` |
| `LLM_EMBEDDINGS_OPENAI_MODEL`         | Embeddings model         | `mxbai-embed-large`         |

Variables can also be loaded from a `.env` file by passing its path to `providerenv.With`.

More advanced options are available. A full runnable example is available in [`example/embedded/`](example/embedded/).

## Sponsors

<a title="Cadoles" href="https://www.cadoles.com">
  <img alt="Logo Cadoles" src="https://www.cadoles.com/images/logo-long.svg" width="200" />
</a>

## License

[AGPL-3.0](LICENSE.md)
