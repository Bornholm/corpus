package mcp

import (
	"context"
	"log/slog"
	"net/http"
	"slices"

	httpCtx "github.com/bornholm/corpus/internal/http/context"
	"github.com/bornholm/corpus/pkg/model"
	"github.com/bornholm/corpus/pkg/port"
	"github.com/bornholm/go-x/slogx"
	"github.com/pkg/errors"
)

func (h *Handler) withParams(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		sessionData := h.getSession(r)

		query := r.URL.Query()
		rawCollections := query["collection"]

		if len(rawCollections) > 0 {
			user := httpCtx.User(ctx)

			readableCollections, _, err := h.documentManager.DocumentStore.QueryUserReadableCollections(ctx, user.ID(), port.QueryCollectionsOptions{})
			if err != nil {
				slog.ErrorContext(ctx, "could not retrieve user readable collections", slogx.Error(err))
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			collections := make([]model.CollectionID, 0, len(rawCollections))
			for _, rawCollectionID := range rawCollections {
				collectionID := model.CollectionID(rawCollectionID)

				isReadable := slices.ContainsFunc(readableCollections, func(c model.PersistedCollection) bool {
					return collectionID == c.ID()
				})

				if !isReadable {
					http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
					return
				}

				collections = append(collections, collectionID)
			}

			sessionData.Collections = collections
			if err := h.saveSession(w, r, sessionData); err != nil {
				slog.ErrorContext(ctx, "could not save session", slog.Any("error", errors.WithStack(err)))
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
		} else if len(sessionData.Collections) == 0 {
			user := httpCtx.User(ctx)

			readableCollections, _, err := h.documentManager.DocumentStore.QueryUserReadableCollections(ctx, user.ID(), port.QueryCollectionsOptions{})
			if err != nil {
				slog.ErrorContext(ctx, "could not retrieve user readable collections", slogx.Error(err))
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			sessionData.Collections = slices.Collect(func(yield func(model.CollectionID) bool) {
				for _, c := range readableCollections {
					if !yield(c.ID()) {
						return
					}
				}
			})

			if err := h.saveSession(w, r, sessionData); err != nil {
				slog.ErrorContext(ctx, "could not save session", slog.Any("error", errors.WithStack(err)))
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
		}

		ctx = context.WithValue(ctx, contextKeySessionData, sessionData)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	}

	return http.HandlerFunc(fn)
}
