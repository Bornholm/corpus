package mcp

import (
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/bornholm/go-x/slogx"
	"github.com/pkg/errors"

	httpCtx "github.com/bornholm/corpus/internal/http/context"
)

func (h *Handler) withParams(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/mcp/sse") {
			next.ServeHTTP(w, r)
			return
		}

		shouldSave := false

		sessionData := h.getSession(r)

		query := r.URL.Query()

		ctx := r.Context()
		user := httpCtx.User(ctx)

		collections := make([]model.CollectionID, 0)

		readableCollections, _, err := h.documentManager.DocumentStore.QueryUserReadableCollections(ctx, user.ID(), port.QueryCollectionsOptions{})
		if err != nil {
			slog.ErrorContext(ctx, "could not retrieve user readable collections", slogx.Error(err))
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		rawCollections := query["collection"]

		if len(rawCollections) > 0 {
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
			shouldSave = true
		} else {

			collections = slices.Collect(func(yield func(model.CollectionID) bool) {
				for _, c := range readableCollections {
					if !yield(c.ID()) {
						return
					}
				}
			})

			shouldSave = true
		}

		if shouldSave {
			if err := h.saveSession(w, r, sessionData); err != nil {
				slog.ErrorContext(r.Context(), "could not save session", slog.Any("error", errors.WithStack(err)))
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
		}

		next.ServeHTTP(w, r)
	}

	return http.HandlerFunc(fn)
}
