package mcp

import (
	"context"
	"slices"
	"strings"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/core/port"
	httpCtx "github.com/bornholm/corpus/internal/http/context"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/pkg/errors"
)

func (h *Handler) handleListCollections(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	user := httpCtx.User(ctx)

	collections, _, err := h.documentManager.DocumentStore.QueryUserReadableCollections(ctx, user.ID(), port.QueryCollectionsOptions{
		HeaderOnly: true,
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	sessionData := contextSessionData(ctx)

	filteredCollections := slices.Collect(func(yield func(c model.Collection) bool) {
		for _, c := range collections {
			if len(sessionData.Collections) == 0 {
				if !yield(c) {
					return
				}
			}

			matches := slices.ContainsFunc(sessionData.Collections, func(id model.CollectionID) bool {
				return id == c.ID()
			})

			if !matches {
				continue
			}

			if !yield(c) {
				return
			}
		}
	})

	var sb strings.Builder

	if len(filteredCollections) == 0 {
		sb.WriteString("No collection available.")
	} else {
		sb.WriteString("## Available collections ")

		for _, c := range filteredCollections {

			sb.WriteString("#### Collection '")
			sb.WriteString(c.Label())
			sb.WriteString("'\n\n")

			sb.WriteString("**ID:** ")
			sb.WriteString(string(c.ID()))
			sb.WriteString("\n\n")

			sb.WriteString("**Description:**\n")
			sb.WriteString(c.Description())
			sb.WriteString("\n\n")
		}

	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: sb.String(),
			},
		},
	}, nil
}
