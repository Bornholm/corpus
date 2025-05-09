package mcp

import (
	"context"
	"net/http"
)

type contextKey string

const (
	contextKeySessionData contextKey = "sessionData"
)

func (h *Handler) updateSessionContext(ctx context.Context, r *http.Request) context.Context {
	sessionData := h.getSession(r)
	ctx = context.WithValue(ctx, contextKeySessionData, sessionData)
	return ctx
}

func contextSessionData(ctx context.Context) *SessionData {
	rawSessionData := ctx.Value(contextKeySessionData)
	if rawSessionData == nil {
		return &SessionData{
			Collections: make([]string, 0),
		}
	}

	if sessionData, ok := rawSessionData.(*SessionData); ok {
		return sessionData
	}

	return &SessionData{
		Collections: make([]string, 0),
	}
}
