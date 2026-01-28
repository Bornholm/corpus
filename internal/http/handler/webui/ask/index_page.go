package ask

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/bornholm/corpus/internal/core/service"
	httpCtx "github.com/bornholm/corpus/internal/http/context"
	"github.com/bornholm/corpus/internal/http/handler/webui/common"
	"github.com/bornholm/go-x/slogx"
	"github.com/pkg/errors"
)

const maxBodySize = 32<<20 + 512

func (h *Handler) handleIndex(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	if err := r.ParseMultipartForm(maxBodySize); err != nil {
		slog.ErrorContext(ctx, "could not parse multipart form", slogx.Error(err))
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		slog.ErrorContext(ctx, "could not read form file", slogx.Error(err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	defer file.Close()

	options := make([]service.DocumentManagerIndexFileOptionFunc, 0)

	rawSource := r.FormValue("source")
	if rawSource != "" {
		source, err := url.Parse(rawSource)
		if err != nil {
			slog.ErrorContext(ctx, "could not parse source url", slogx.Error(err))
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		options = append(options, service.WithDocumentManagerIndexFileSource(source))
	}

	if rawCollections, exists := r.Form["collection"]; exists {
		collections, err := h.assertWritableCollections(ctx, rawCollections)
		if err != nil {
			slog.ErrorContext(ctx, "could not assert writable collections", slogx.Error(err))
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		options = append(options, service.WithDocumentManagerIndexFileCollections(collections...))
	}

	slog.DebugContext(ctx, "indexing uploaded document")

	user := httpCtx.User(ctx)

	taskID, err := h.documentManager.IndexFile(ctx, user, fileHeader.Filename, file, options...)
	if err != nil {
		slog.ErrorContext(ctx, "could not index uploaded file", slogx.Error(err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	baseURL := httpCtx.BaseURL(ctx)

	taskURL := baseURL.JoinPath(fmt.Sprintf("/tasks/%s", taskID))

	http.Redirect(w, r, taskURL.String(), http.StatusSeeOther)
}

func (h *Handler) assertWritableCollections(ctx context.Context, rawCollections []string) ([]model.CollectionID, error) {
	user := httpCtx.User(ctx)

	writableCollections, _, err := h.documentManager.DocumentStore.QueryUserWritableCollections(ctx, user.ID(), port.QueryCollectionsOptions{})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	// If user has writable collections but none are selected, return an error
	if len(writableCollections) > 0 && len(rawCollections) == 0 {
		return nil, common.NewHTTPError(http.StatusBadRequest)
	}

	collections := make([]model.CollectionID, 0)

	if len(rawCollections) > 0 {
		for _, rawCollectionID := range rawCollections {
			collectionID := model.CollectionID(rawCollectionID)

			isWritable := slices.ContainsFunc(writableCollections, func(c model.PersistedCollection) bool {
				return collectionID == c.ID()
			})

			if !isWritable {
				return nil, common.NewHTTPError(http.StatusForbidden)
			}

			collections = append(collections, collectionID)
		}
	} else {
		// If no writable collections exist, allow empty collections (will be handled by document manager)
		collections = slices.Collect(func(yield func(model.CollectionID) bool) {
			for _, c := range writableCollections {
				if !yield(c.ID()) {
					return
				}
			}
		})
	}

	return collections, nil
}
