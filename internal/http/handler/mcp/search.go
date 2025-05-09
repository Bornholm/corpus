package mcp

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/bornholm/corpus/internal/core/port"
	"github.com/bornholm/corpus/internal/core/service"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/pkg/errors"
)

func (h *Handler) handleSearch(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	_, results, err := h.doSearch(ctx, request)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	content := make([]mcp.Content, 0)

	if len(results) == 0 {
		content = append(content, mcp.TextContent{
			Type: "text",
			Text: "No result available matching the given query.",
		})

		return &mcp.CallToolResult{
			Content: content,
		}, nil
	}

	var sb strings.Builder

	for _, r := range results {
		sb.Reset()
		sb.WriteString("# Result\n\n")
		sb.WriteString("**URL** ")
		sb.WriteString(r.Source.String())
		sb.WriteString("\n\n")

		sb.WriteString("## Excerpts\n\n")

		for i, sectionID := range r.Sections {
			if i > 0 {
				sb.WriteString("\n\n[...]\n\n")
			}

			section, err := h.documentManager.GetSectionByID(ctx, sectionID)
			if err != nil {
				return nil, errors.WithStack(err)
			}

			content, err := section.Content()
			if err != nil {
				return nil, errors.WithStack(err)
			}

			sb.Write(content)
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

func (h *Handler) doSearch(ctx context.Context, request mcp.CallToolRequest) (string, []*port.IndexSearchResult, error) {
	arguments := request.Params.Arguments
	query, ok := arguments["query"].(string)
	if !ok {
		return "", nil, fmt.Errorf("invalid query argument")
	}

	options := make([]service.DocumentManagerSearchOptionFunc, 0)

	sessionData := contextSessionData(ctx)

	rawCollection, exists := arguments["collection"]
	if exists {
		collection, ok := rawCollection.(string)
		if !ok {
			return "", nil, fmt.Errorf("invalid collection argument")
		}

		options = append(options, service.WithDocumentManagerSearchCollections(collection))

		if len(sessionData.Collections) == 0 || slices.Contains(sessionData.Collections, collection) {
			options = append(options, service.WithDocumentManagerSearchCollections(collection))
		} else {
			options = append(options, service.WithDocumentManagerSearchCollections(sessionData.Collections...))
		}
	} else if len(sessionData.Collections) > 0 {
		options = append(options, service.WithDocumentManagerSearchCollections(sessionData.Collections...))
	}

	results, err := h.documentManager.Search(ctx, query, options...)
	if err != nil {
		return "", nil, errors.WithStack(err)
	}

	return "", results, nil
}
