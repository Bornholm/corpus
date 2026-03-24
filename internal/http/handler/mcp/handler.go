package mcp

import (
	"context"
	"crypto/rand"
	"log/slog"
	"net/http"
	"time"

	"github.com/bornholm/corpus/internal/build"
	"github.com/bornholm/corpus/internal/core/service"
	"github.com/bornholm/corpus/internal/http/middleware/authz"
	"github.com/gorilla/sessions"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/pkg/errors"
)

type Handler struct {
	documentManager *service.DocumentManager
	basePath        string
	handler         http.Handler
	sessions        sessions.Store
	mcp             *server.MCPServer
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	slog.DebugContext(r.Context(), "mcp handler received request",
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
		slog.String("query", r.URL.RawQuery),
		slog.Int64("contentLength", r.ContentLength),
	)

	currentPath := r.URL.Path
	r.URL.Path = h.basePath
	r.URL = r.URL.JoinPath(currentPath)
	h.handler.ServeHTTP(w, r)
}

func NewHandler(baseURL string, basePath string, documentManager *service.DocumentManager) *Handler {
	signingKey, err := getCookieSigningKey()
	if err != nil {
		panic(errors.Wrap(err, "could not generate cookie signing key"))
	}

	h := &Handler{
		documentManager: documentManager,
		basePath:        basePath,
		sessions:        sessions.NewCookieStore(signingKey),
	}

	mcpServer := server.NewMCPServer("corpus", build.ShortVersion,
		server.WithToolCapabilities(true),
		server.WithToolHandlerMiddleware(func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
			return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				slog.DebugContext(ctx, "mcp tool call", slog.String("tool_name", request.Request.Method), slog.Any("tool_params", request.Request.Params))
				return next(ctx, request)
			}
		}),
	)

	mcpServer.AddTool(getAskTool(), h.handleAsk)
	mcpServer.AddTool(getListCollectionsTool(), h.handleListCollections)

	h.mcp = mcpServer

	sseServer := server.NewSSEServer(
		mcpServer,
		server.WithBaseURL(baseURL),
		server.WithStaticBasePath(basePath),
		server.WithHTTPContextFunc(h.updateSessionContext),
		server.WithKeepAlive(true),
		server.WithKeepAliveInterval(15*time.Second),
	)

	assertAuthenticated := authz.Middleware(nil, authz.IsAuthenticated)

	h.handler = assertAuthenticated(h.withParams(sseServer))

	return h
}

func getAskTool() mcp.Tool {
	return mcp.NewTool("ask",
		mcp.WithDescription(`Use this tool to ask questions and get answers from indexed documents using Retrieval-Augmented Generation (RAG). This tool searches the knowledge base for relevant context and generates a natural language response based on the found information.

**When to use:**
- When you need to find specific information about a topic covered by the document corpus
- When you want to summarize or extract information from the document corpus
- When the user asks "what", "how", "why", "who", "where" questions about stored content

**Note:** If no relevant documents are found, the tool will return an error indicating no matching information was available.`),
		mcp.WithString("question",
			mcp.Description(`The question to ask the knowledge base. For best results:
- Use complete, natural language sentences
- Be specific about what information you're looking for
- Include relevant context if needed
- Always use the language of the user
- Examples: "What is the company's return policy for electronics?", "Quelle est la date de création de l'association ?"`),
			mcp.Required(),
		),
		mcp.WithString("collection",
			mcp.Description(`Optional. The collection ID to restrict the search to. If not provided, searches across all collections the user has access to.

**How to get collection IDs:** Use the 'list_collections' tool first to retrieve available collections with their IDs.

**Example collection ID format:** "c9v3jk2p3n4f9d8e7h6g5r4t3" (xid format)`),
		),
	)
}

func getListCollectionsTool() mcp.Tool {
	return mcp.NewTool("list_collections",
		mcp.WithDescription(`List all document collections available to the authenticated user.

**When to use:**
- Before using the 'ask' tool when you need to know which collections are available
- To retrieve collection IDs for the 'collection' parameter in the 'ask' tool
- To show users what document sets they can query

**Returns:** A list of collections with their ID (use this in the 'collection' parameter), label (display name), and description.`),
	)
}

func getCookieSigningKey() ([]byte, error) {
	key := make([]byte, 32)

	if _, err := rand.Read(key); err != nil {
		return nil, errors.WithStack(err)
	}

	return key, nil
}

var _ http.Handler = &Handler{}
