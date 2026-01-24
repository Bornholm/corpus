package api

import (
	"net/http"
	"net/url"
	"slices"
	"strconv"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/core/port"
	httpCtx "github.com/bornholm/corpus/internal/http/context"
	"github.com/bornholm/corpus/internal/http/handler/webui/common"
	"github.com/pkg/errors"
)

func (h *Handler) getSelectedCollectionsFromRequest(r *http.Request) ([]model.CollectionID, error) {
	ctx := r.Context()
	user := httpCtx.User(ctx)

	readableCollections, _, err := h.documentManager.DocumentStore.QueryUserReadableCollections(ctx, user.ID(), port.QueryCollectionsOptions{})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	rawCollections := r.URL.Query()["collection"]

	collections := make([]model.CollectionID, 0)

	if len(rawCollections) > 0 {
		for _, rawCollectionID := range rawCollections {
			collectionID := model.CollectionID(rawCollectionID)

			isReadable := slices.ContainsFunc(readableCollections, func(c model.PersistedCollection) bool {
				return collectionID == c.ID()
			})

			if !isReadable {
				return nil, common.NewHTTPError(http.StatusForbidden)
			}

			collections = append(collections, collectionID)
		}
	} else {
		collections = slices.Collect(func(yield func(model.CollectionID) bool) {
			for _, c := range readableCollections {
				if !yield(c.ID()) {
					return
				}
			}
		})
	}

	return collections, nil
}

func getQueryPage(query url.Values, defaultValue int) int {
	return getQueryInt(query, "page", defaultValue)
}

func getQueryLimit(query url.Values, defaultValue int) int {
	return getQueryInt(query, "limit", defaultValue)
}

func getQueryInt(query url.Values, name string, defaultValue int) int {
	raw := query.Get(name)
	if raw == "" {
		return defaultValue
	}

	value, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		return defaultValue
	}

	return int(value)
}
