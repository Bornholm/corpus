package mcp

import (
	"context"
	"log/slog"
	"slices"
	"strconv"
	"strings"

	httpCtx "github.com/bornholm/corpus/internal/http/context"
	"github.com/bornholm/corpus/pkg/model"
	"github.com/bornholm/corpus/pkg/port"
	"github.com/bornholm/go-x/slogx"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/pkg/errors"
)

func (h *Handler) handleListCollections(ctx context.Context, request *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
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

			stats, err := h.documentManager.DocumentStore.GetCollectionStats(ctx, c.ID())
			if err != nil {
				slog.WarnContext(ctx, "could not retrieve collection stats", slog.String("collectionID", string(c.ID())), slogx.Error(err))
			} else {
				sb.WriteString("**Total documents:** ")
				sb.WriteString(strconv.FormatInt(stats.TotalDocuments, 10))
				sb.WriteString("\n\n")
			}

			sb.WriteString("**Description:**\n")
			sb.WriteString(c.Description())
			sb.WriteString("\n\n")

		}

	}

	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{
			&sdkmcp.TextContent{Text: sb.String()},
		},
	}, nil
}
