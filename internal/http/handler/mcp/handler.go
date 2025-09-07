package mcp

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"strings"

	"github.com/bornholm/corpus/internal/build"
	"github.com/bornholm/corpus/internal/core/port"
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
		server.WithToolFilter(h.filterTools),
	)

	mcpServer.AddTool(getAskTool(defaultAskDescription), h.handleAsk)

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

const defaultAskDescription string = "Ask a properly formulated question to the knowledge base."

func getAskTool(description string) mcp.Tool {
	return mcp.NewTool("ask",
		mcp.WithDescription(description),
		mcp.WithString("question",
			mcp.Description(`A properly formulated question to submit to the knowledge base.`),
			mcp.Required(),
		),
	)
}

func (h *Handler) filterTools(ctx context.Context, tools []mcp.Tool) []mcp.Tool {
	askDescription, err := h.getAskDescription(ctx)
	if err != nil {
		panic(errors.WithStack(err))
	}

	tools = []mcp.Tool{
		getAskTool(askDescription),
	}

	return tools
}

func (h *Handler) getAskDescription(ctx context.Context) (string, error) {
	collections, err := h.documentManager.Store.QueryCollections(ctx, port.QueryCollectionsOptions{})
	if err != nil {
		return "", errors.WithStack(err)
	}

	var sb strings.Builder

	sb.WriteString("## Available collections ")

	for _, c := range collections {
		sb.WriteString("#### Collection '")
		sb.WriteString(c.Label())
		sb.WriteString("'\n\n")

		sb.WriteString("**ID:** ")
		sb.WriteString(c.Name())
		sb.WriteString("\n\n")

		sb.WriteString("**Description:**\n")
		sb.WriteString(c.Description())
		sb.WriteString("\n\n")
	}

	return fmt.Sprintf(`
	%s

	%s
	`, defaultAskDescription, sb.String()), nil
}

func getCookieSigningKey() ([]byte, error) {
	key := make([]byte, 32)

	if _, err := rand.Read(key); err != nil {
		return nil, errors.WithStack(err)
	}

	return key, nil
}

var _ http.Handler = &Handler{}
