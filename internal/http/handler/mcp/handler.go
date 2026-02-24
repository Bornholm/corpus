package mcp

import (
	"context"
	"crypto/rand"
	"log/slog"
	"net/http"

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
	)

	assertAuthenticated := authz.Middleware(nil, authz.IsAuthenticated)

	h.handler = assertAuthenticated(h.withParams(sseServer))

	return h
}

func getAskTool() mcp.Tool {
	return mcp.NewTool("ask",
		mcp.WithDescription("Ask a question to the knowledge database about a topic"),
		mcp.WithString("question",
			mcp.Description(`A properly formulated question to submit to the knowledge base.`),
			mcp.Required(),
		),
		mcp.WithString("collection",
			mcp.Description("Collection identifier. Restrict the knowledge base search to this collection"),
		),
	)
}

func getListCollectionsTool() mcp.Tool {
	return mcp.NewTool("list_collections",
		mcp.WithDescription("List available documents collections in the knowledge database"),
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
