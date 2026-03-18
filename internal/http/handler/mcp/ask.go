package mcp

import (
	"context"
	"slices"
	"strings"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/bornholm/corpus/internal/core/service"
	httpCtx "github.com/bornholm/corpus/internal/http/context"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/pkg/errors"
)

func (h *Handler) handleAsk(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	arguments := request.Params.Arguments

	question, ok := arguments["question"].(string)
	if !ok {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: "The 'question' required argument is missing.",
				},
			},
			IsError: true,
		}, nil
	}

	query, results, err := h.doSearch(ctx, question)
	if err != nil {
		var invalidCollectionErr InvalidCollectionError
		if errors.As(err, &invalidCollectionErr) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					mcp.TextContent{
						Type: "text",
						Text: invalidCollectionErr.Error(),
					},
				},
				IsError: true,
			}, nil
		}

		return nil, errors.WithStack(err)
	}

	content := make([]mcp.Content, 0)

	if len(results) == 0 {
		content = append(content, mcp.TextContent{
			Type: "text",
			Text: "No information available matching the given question.",
		})

		return &mcp.CallToolResult{
			Content: content,
			IsError: true,
		}, nil
	}

	response, sections, err := h.documentManager.Ask(ctx, query, results)
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

func (h *Handler) doSearch(ctx context.Context, query string) (string, []*port.IndexSearchResult, error) {
	options := make([]service.DocumentManagerSearchOptionFunc, 0)

	sessionData := contextSessionData(ctx)

	if len(sessionData.Collections) > 0 {
		// Validate that all provided collection IDs are valid
		user := httpCtx.User(ctx)
		readableCollections, _, err := h.documentManager.DocumentStore.QueryUserReadableCollections(ctx, user.ID(), port.QueryCollectionsOptions{
			HeaderOnly: true,
		})
		if err != nil {
			return "", nil, errors.WithStack(err)
		}

		// Check if any of the session collections are invalid
		var invalidCollections []model.CollectionID
		for _, collectionID := range sessionData.Collections {
			isValid := slices.ContainsFunc(readableCollections, func(c model.PersistedCollection) bool {
				return collectionID == c.ID()
			})
			if !isValid {
				invalidCollections = append(invalidCollections, collectionID)
			}
		}

		if len(invalidCollections) > 0 {
			return "", nil, InvalidCollectionError{
				InvalidCollections: invalidCollections,
			}
		}

		options = append(options, service.WithDocumentManagerSearchCollections(sessionData.Collections...))
	}

	results, err := h.documentManager.Search(ctx, query, options...)
	if err != nil {
		return "", nil, errors.WithStack(err)
	}

	return "", results, nil
}

// InvalidCollectionError is returned when an invalid collection identifier is provided.
type InvalidCollectionError struct {
	InvalidCollections []model.CollectionID
}

func (e InvalidCollectionError) Error() string {
	var sb strings.Builder
	sb.WriteString("invalid collection identifier(s): ")
	for i, id := range e.InvalidCollections {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(string(id))
	}
	sb.WriteString(". Please provide a valid collection ID (e.g., ")
	sb.WriteString(string(e.InvalidCollections[0]))
	sb.WriteString("). Collection identifiers must be the collection ID, not the name or slug. Use the 'list_collections' tool to retrieve available collection IDs.")
	return sb.String()
}
