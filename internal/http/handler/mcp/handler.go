package mcp

import (
	"crypto/rand"
	"net/http"

	"github.com/bornholm/corpus/internal/build"
	"github.com/bornholm/corpus/internal/core/service"
	"github.com/bornholm/corpus/internal/http/authz"
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
	)

	mcpServer.AddTool(mcp.NewTool("ask",
		mcp.WithDescription("Ask a question and retrieve a response generated from the available informations in the knowledge base"),
		mcp.WithString("query",
			mcp.Description("The query to submit to the knowledge base"),
			mcp.Required(),
		),
		mcp.WithString("collection",
			mcp.Description("The collection ID to restrict the query to"),
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

	h.mcp = mcpServer

	sseServer := server.NewSSEServer(
		mcpServer,
		server.WithBaseURL(baseURL),
		server.WithBasePath(basePath),
		server.WithSSEContextFunc(h.updateSessionContext),
	)

	assertAuthenticated := authz.Middleware(authz.IsAuthenticated)

	h.handler = assertAuthenticated(h.withParams(sseServer))

	return h
}

func getCookieSigningKey() ([]byte, error) {
	key := make([]byte, 32)

	if _, err := rand.Read(key); err != nil {
		return nil, errors.WithStack(err)
	}

	return key, nil
}

var _ http.Handler = &Handler{}
