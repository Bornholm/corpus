package mcp

import (
	"context"

	"github.com/bornholm/corpus/pkg/model"
)

type contextKey string

const (
	contextKeySessionData contextKey = "sessionData"
)

func contextSessionData(ctx context.Context) *SessionData {
	rawSessionData := ctx.Value(contextKeySessionData)
	if rawSessionData == nil {
		return &SessionData{
			Collections: make([]model.CollectionID, 0),
		}
	}

	if sessionData, ok := rawSessionData.(*SessionData); ok {
		return sessionData
	}

	return &SessionData{
		Collections: make([]model.CollectionID, 0),
	}
}
