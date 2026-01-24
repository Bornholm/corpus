package api

import (
	"context"
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

func (h *Handler) handleIndexDocument(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
	if err := r.ParseMultipartForm(maxBodySize); err != nil {
		slog.ErrorContext(ctx, "could not parse multipart form", slogx.Error(err))
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	defer r.Body.Close()

	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		slog.ErrorContext(ctx, "could not read form file", slogx.Error(err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	defer file.Close()

	options := make([]service.DocumentManagerIndexFileOptionFunc, 0)

	if rawSource := r.FormValue("source"); rawSource != "" {
		source, err := url.Parse(rawSource)
		if err != nil {
			slog.ErrorContext(ctx, "could not parse source url", slogx.Error(err))
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		options = append(options, service.WithDocumentManagerIndexFileSource(source))
	}

	if etag := r.FormValue("etag"); etag != "" {
		options = append(options, service.WithDocumentManagerIndexFileETag(etag))
	}

	if rawCollections, exists := r.Form["collection"]; exists {
		collections, err := h.assertWritableCollections(ctx, rawCollections)
		if err != nil {
			var httpErr common.HTTPError
			if errors.As(err, &httpErr) {
				http.Error(w, http.StatusText(httpErr.StatusCode()), httpErr.StatusCode())
				return
			}

			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		options = append(options, service.WithDocumentManagerIndexFileCollections(collections...))
	}

	slog.DebugContext(ctx, "indexing uploaded document")

	user := httpCtx.User(ctx)

	taskID, err := h.documentManager.IndexFile(ctx, user.ID(), fileHeader.Filename, file, options...)
	if err != nil {
		slog.ErrorContext(ctx, "could not index uploaded file", slogx.Error(err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	h.writeTask(ctx, w, taskID)
}

func (h *Handler) assertWritableCollections(ctx context.Context, rawCollections []string) ([]model.CollectionID, error) {
	user := httpCtx.User(ctx)

	writableCollections, _, err := h.documentManager.DocumentStore.QueryUserWritableCollections(ctx, user.ID(), port.QueryCollectionsOptions{})
	if err != nil {
		return nil, errors.WithStack(err)
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
