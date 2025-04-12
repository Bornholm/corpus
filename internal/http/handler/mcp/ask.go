package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/bornholm/corpus/internal/core/service"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/pkg/errors"
)

func (h *Handler) handleAsk(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	arguments := request.Params.Arguments
	question, ok := arguments["question"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid question argument")
	}

	options := make([]service.DocumentManagerSearchOptionFunc, 0)

	rawCollection, exists := arguments["collection"]
	if exists {
		collection, ok := rawCollection.(string)
		if !ok {
			return nil, fmt.Errorf("invalid collection argument")
		}

		options = append(options, service.WithDocumentManagerSearchCollections(collection))
	}

	results, err := h.documentManager.Search(ctx, question, options...)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	content := make([]mcp.Content, 0)

	if len(results) == 0 {
		content = append(content, mcp.TextContent{
			Type: "text",
			Text: "No result available matching the given question.",
		})

		return &mcp.CallToolResult{
			Content: content,
		}, nil
	}

	response, sections, err := h.documentManager.Ask(ctx, question, results)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var sb strings.Builder

	sb.WriteString("# Response \n\n")
	sb.WriteString(response)

	content = append(content, mcp.TextContent{
		Type: "text",
		Text: sb.String(),
	})

	for _, r := range results {
		sb.Reset()
		sb.WriteString("# Source\n\n")
		sb.WriteString("**URL** ")
		sb.WriteString(r.Source.String())
		sb.WriteString("\n\n")

		sb.WriteString("## Excerpts\n\n")

		for i, sectionID := range r.Sections {
			content, exists := sections[sectionID]
			if !exists {
				continue
			}

			if i > 0 {
				sb.WriteString("\n\n[...]\n\n")
			}

			sb.WriteString(content)
		}

		content = append(content, mcp.TextContent{
			Type: "text",
			Text: sb.String(),
		})
	}

	return &mcp.CallToolResult{
		Content: content,
	}, nil
}
