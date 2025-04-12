package mcp

import (
	"context"
	"strings"

	"github.com/bornholm/corpus/internal/core/port"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/pkg/errors"
)

func (h *Handler) handleListCollections(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	collections, err := h.documentManager.Store.QueryCollections(ctx, port.QueryCollectionsOptions{})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	content := make([]mcp.Content, 0)

	var sb strings.Builder

	for _, c := range collections {
		sb.Reset()
		sb.WriteString("# ")
		sb.WriteString(c.Label())
		sb.WriteString("\n\n")

		sb.WriteString("**ID:** ")
		sb.WriteString(c.Name())
		sb.WriteString("\n\n")

		sb.WriteString(c.Description())

		content = append(content, mcp.TextContent{
			Type: "text",
			Text: sb.String(),
		})
	}

	return &mcp.CallToolResult{
		Content: content,
	}, nil
}
