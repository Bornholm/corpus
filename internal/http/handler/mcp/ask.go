package mcp

import (
	"context"
	"encoding/json"
	"slices"
	"strings"

	httpCtx "github.com/bornholm/corpus/internal/http/context"
	"github.com/bornholm/corpus/pkg/model"
	"github.com/bornholm/corpus/pkg/port"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/pkg/errors"
)

func (h *Handler) handleAsk(ctx context.Context, request *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
	var args map[string]any
	if err := json.Unmarshal(request.Params.Arguments, &args); err != nil {
		return &sdkmcp.CallToolResult{
			Content: []sdkmcp.Content{
				&sdkmcp.TextContent{Text: "Invalid arguments: " + err.Error()},
			},
			IsError: true,
		}, nil
	}

	question, ok := args["question"].(string)
	if !ok || question == "" {
		return &sdkmcp.CallToolResult{
			Content: []sdkmcp.Content{
				&sdkmcp.TextContent{Text: "The 'question' required argument is missing."},
			},
			IsError: true,
		}, nil
	}

	collections, err := h.resolveSessionCollections(ctx)
	if err != nil {
		var invalidCollectionErr InvalidCollectionError
		if errors.As(err, &invalidCollectionErr) {
			return &sdkmcp.CallToolResult{
				Content: []sdkmcp.Content{
					&sdkmcp.TextContent{Text: invalidCollectionErr.Error()},
				},
				IsError: true,
			}, nil
		}

		return nil, errors.WithStack(err)
	}

	result, err := h.documentManager.AskWithRetrieval(ctx, question, collections)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	content := make([]sdkmcp.Content, 0)

	if len(result.Results) == 0 {
		content = append(content, &sdkmcp.TextContent{
			Text: "No information available matching the given question.",
		})

		return &sdkmcp.CallToolResult{
			Content: content,
			IsError: true,
		}, nil
	}

	var sb strings.Builder

	sb.WriteString("# Response \n\n")
	sb.WriteString(result.Answer)

	content = append(content, &sdkmcp.TextContent{
		Text: sb.String(),
	})

	for _, r := range result.Results {
		sb.Reset()
		sb.WriteString("# Source\n\n")
		sb.WriteString("**URL** ")
		sb.WriteString(r.Source.String())
		sb.WriteString("\n\n")

		sb.WriteString("## Excerpts\n\n")

		for i, sectionID := range r.Sections {
			sectionContent, exists := result.Contents[sectionID]
			if !exists {
				continue
			}

			if i > 0 {
				sb.WriteString("\n\n[...]\n\n")
			}

			sb.WriteString(sectionContent)
		}

		content = append(content, &sdkmcp.TextContent{
			Text: sb.String(),
		})
	}

	return &sdkmcp.CallToolResult{
		Content: content,
	}, nil
}

// resolveSessionCollections returns the (validated) collection IDs the current
// MCP session restricts search to. An empty slice means "no restriction".
func (h *Handler) resolveSessionCollections(ctx context.Context) ([]model.CollectionID, error) {
	sessionData := contextSessionData(ctx)

	if len(sessionData.Collections) == 0 {
		return nil, nil
	}

	// Validate that all provided collection IDs are valid
	user := httpCtx.User(ctx)
	readableCollections, _, err := h.documentManager.DocumentStore.QueryUserReadableCollections(ctx, user.ID(), port.QueryCollectionsOptions{
		HeaderOnly: true,
	})
	if err != nil {
		return nil, errors.WithStack(err)
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
		return nil, InvalidCollectionError{
			InvalidCollections: invalidCollections,
		}
	}

	return sessionData.Collections, nil
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
