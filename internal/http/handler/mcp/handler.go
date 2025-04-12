package mcp

import (
	"net/http"

	"github.com/bornholm/corpus/internal/build"
	"github.com/bornholm/corpus/internal/core/service"
	"github.com/bornholm/corpus/internal/http/authz"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type Handler struct {
	documentManager *service.DocumentManager
	basePath        string
	handler         http.Handler
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	currentPath := r.URL.Path
	r.URL.Path = h.basePath
	r.URL = r.URL.JoinPath(currentPath)
	h.handler.ServeHTTP(w, r)
}

func NewHandler(baseURL string, basePath string, documentManager *service.DocumentManager) *Handler {
	h := &Handler{
		documentManager: documentManager,
		basePath:        basePath,
	}

	mcpServer := server.NewMCPServer("corpus", build.ShortVersion,
		server.WithToolCapabilities(true),
	)

	mcpServer.AddTool(mcp.NewTool("ask",
		mcp.WithDescription("Ask a question and retrieve a response generated from the available informations in the knowledge base"),
		mcp.WithString("question",
			mcp.Description("The question to submit to the knowledge base"),
			mcp.Required(),
		),
		mcp.WithString("collection",
			mcp.Description("The collection ID to restrict the question to"),
		),
	), h.handleAsk)

	mcpServer.AddTool(mcp.NewTool("search",
		mcp.WithDescription("Search for informations in the knowledge base"),
		mcp.WithString("query",
			mcp.Description("The query to submit to the knowledge base"),
			mcp.Required(),
		),
		mcp.WithString("collection",
			mcp.Description("The collection ID to restrict the query to"),
		),
	), h.handleSearch)

	mcpServer.AddTool(mcp.NewTool("list_collections",
		mcp.WithDescription("List the collection of documents available in the knowledge base"),
	), h.handleListCollections)

	sseServer := server.NewSSEServer(
		mcpServer,
		server.WithBaseURL(baseURL),
		server.WithBasePath(basePath),
	)

	assertAuthenticated := authz.Middleware(authz.IsAuthenticated)

	h.handler = assertAuthenticated(sseServer)

	return h
}

var _ http.Handler = &Handler{}
