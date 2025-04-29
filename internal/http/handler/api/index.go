package api

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/bornholm/corpus/internal/core/service"
	httpCtx "github.com/bornholm/corpus/internal/http/context"
	"github.com/pkg/errors"
)

const maxBodySize = 32<<20 + 512

func (h *Handler) handleIndexDocument(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
	if err := r.ParseMultipartForm(maxBodySize); err != nil {
		slog.ErrorContext(ctx, "could not parse multipart form", slog.Any("error", errors.WithStack(err)))
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	defer r.Body.Close()

	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		slog.ErrorContext(ctx, "could not read form file", slog.Any("error", errors.WithStack(err)))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	defer file.Close()

	options := make([]service.DocumentManagerIndexFileOptionFunc, 0)

	if rawSource := r.FormValue("source"); rawSource != "" {
		source, err := url.Parse(rawSource)
		if err != nil {
			slog.ErrorContext(ctx, "could not parse source url", slog.Any("error", errors.WithStack(err)))
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		options = append(options, service.WithDocumentManagerIndexFileSource(source))
	}

	if etag := r.FormValue("etag"); etag != "" {
		options = append(options, service.WithDocumentManagerIndexFileETag(etag))
	}

	if collections, exists := r.Form["collection"]; exists {
		options = append(options, service.WithDocumentManagerIndexFileCollections(collections...))
	}

	slog.DebugContext(ctx, "indexing uploaded document")

	taskID, err := h.documentManager.IndexFile(ctx, fileHeader.Filename, file, options...)
	if err != nil {
		slog.ErrorContext(ctx, "could not index uploaded file", slog.Any("error", errors.WithStack(err)))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	baseURL := httpCtx.BaseURL(ctx)
	taskURL := baseURL.JoinPath(fmt.Sprintf("/api/v1/tasks/%s", taskID))

	http.Redirect(w, r, taskURL.String(), http.StatusSeeOther)
}
