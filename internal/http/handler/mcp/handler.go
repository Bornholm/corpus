package mcp

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/bornholm/corpus/internal/build"
	"github.com/bornholm/corpus/internal/core/service"
	"github.com/bornholm/corpus/internal/http/middleware/authz"
	"github.com/gorilla/sessions"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/pkg/errors"
)

type Handler struct {
	documentManager *service.DocumentManager
	handler         http.Handler
	sessions        sessions.Store
	mcp             *sdkmcp.Server
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	slog.DebugContext(r.Context(), "mcp handler received request",
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
		slog.String("query", r.URL.RawQuery),
		slog.Int64("contentLength", r.ContentLength),
	)

	h.handler.ServeHTTP(w, r)
}

func NewHandler(baseURL string, basePath string, documentManager *service.DocumentManager) *Handler {
	signingKey, err := getCookieSigningKey()
	if err != nil {
		panic(errors.Wrap(err, "could not generate cookie signing key"))
	}

	h := &Handler{
		documentManager: documentManager,
		sessions:        sessions.NewCookieStore(signingKey),
	}

	mcpServer := sdkmcp.NewServer(&sdkmcp.Implementation{
		Name:    "corpus",
		Version: build.ShortVersion,
	}, nil)

	mcpServer.AddTool(getAskTool(), h.handleAsk)
	mcpServer.AddTool(getListCollectionsTool(), h.handleListCollections)
	mcpServer.AddReceivingMiddleware(h.loggingMiddleware)

	h.mcp = mcpServer

	// Streamable HTTP transport (MCP spec 2025) — endpoint: /mcp/
	streamableHandler := sdkmcp.NewStreamableHTTPHandler(func(r *http.Request) *sdkmcp.Server {
		return h.mcp
	}, nil)

	// SSE transport (MCP spec 2024-11-05) — endpoint: /mcp/sse
	// The SSEHandler manages both the SSE stream (GET /sse) and message
	// endpoints (POST /sse/{sessionId}) internally.
	sseHandler := sdkmcp.NewSSEHandler(func(r *http.Request) *sdkmcp.Server {
		return h.mcp
	}, nil)

	assertAuthenticated := authz.Middleware(nil, authz.IsAuthenticated)

	// Internal routing: /sse → SSE transport, everything else → Streamable HTTP.
	// Note: the /mcp prefix is already stripped by the server's mount middleware.
	innerMux := http.NewServeMux()
	innerMux.Handle("/sse", assertAuthenticated(h.withParams(sseHandler)))
	innerMux.Handle("/sse/", assertAuthenticated(h.withParams(sseHandler)))
	innerMux.Handle("/", assertAuthenticated(h.withParams(streamableHandler)))

	h.handler = innerMux

	return h
}

func (h *Handler) loggingMiddleware(next sdkmcp.MethodHandler) sdkmcp.MethodHandler {
	return func(ctx context.Context, method string, req sdkmcp.Request) (sdkmcp.Result, error) {
		slog.DebugContext(ctx, "mcp method call", slog.String("method", method))
		return next(ctx, method, req)
	}
}

func getAskTool() *sdkmcp.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"question": {
				"type": "string",
				"description": "The question to ask the knowledge base. For best results:\n- Use complete, natural language sentences\n- Be specific about what information you're looking for\n- Include relevant context if needed\n- Always use the language of the user\n- Examples: \"What is the company's return policy for electronics?\", \"Quelle est la date de création de l'association ?\""
			},
			"collection": {
				"type": "string",
				"description": "Optional. The collection ID to restrict the search to. If not provided, searches across all collections the user has access to.\n\n**How to get collection IDs:** Use the 'list_collections' tool first to retrieve available collections with their IDs.\n\n**Example collection ID format:** \"c9v3jk2p3n4f9d8e7h6g5r4t3\" (xid format)"
			}
		},
		"required": ["question"]
	}`)
	return &sdkmcp.Tool{
		Name: "ask",
		Description: `Use this tool to ask questions and get answers from indexed documents using Retrieval-Augmented Generation (RAG). This tool searches the knowledge base for relevant context and generates a natural language response based on the found information.

**When to use:**
- When you need to find specific information about a topic covered by the document corpus
- When you want to summarize or extract information from the document corpus
- When the user asks "what", "how", "why", "who", "where" questions about stored content

**Note:** If no relevant documents are found, the tool will return an error indicating no matching information was available.`,
		InputSchema: schema,
	}
}

func getListCollectionsTool() *sdkmcp.Tool {
	return &sdkmcp.Tool{
		Name: "list_collections",
		Description: `List all document collections available to the authenticated user.

**When to use:**
- Before using the 'ask' tool when you need to know which collections are available
- To retrieve collection IDs for the 'collection' parameter in the 'ask' tool
- To show users what document sets they can query

**Returns:** A list of collections with their ID (use this in the 'collection' parameter), label (display name), and description.`,
		InputSchema: json.RawMessage(`{"type": "object", "properties": {}}`),
	}
}

func getCookieSigningKey() ([]byte, error) {
	key := make([]byte, 32)

	if _, err := rand.Read(key); err != nil {
		return nil, errors.WithStack(err)
	}

	return key, nil
}

var _ http.Handler = &Handler{}
