package collection

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/core/service"
	httpCtx "github.com/bornholm/corpus/internal/http/context"
	"github.com/bornholm/corpus/internal/http/handler/webui/common"
	"github.com/bornholm/go-x/slogx"
	"github.com/pkg/errors"
)

const maxBodySize = 32<<20 + 512

func (h *Handler) handleIndex(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	collectionID := model.CollectionID(r.PathValue("collectionID"))
	if collectionID == "" {
		common.HandleError(w, r, errors.New("collection ID is required"))
		return
	}

	user := httpCtx.User(ctx)

	canWrite, err := h.documentManager.DocumentStore.CanWriteCollection(ctx, user.ID(), collectionID)
	if err != nil {
		slog.ErrorContext(ctx, "could not parse multipart form", slogx.Error(err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if !canWrite {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}

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

	options = append(options, service.WithDocumentManagerIndexFileCollections(collectionID))

	slog.DebugContext(ctx, "indexing uploaded document")

	taskID, err := h.documentManager.IndexFile(ctx, user, fileHeader.Filename, file, options...)
	if err != nil {
		slog.ErrorContext(ctx, "could not index uploaded file", slogx.Error(err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	baseURL := httpCtx.BaseURL(ctx)

	taskURL := baseURL.JoinPath(fmt.Sprintf("/collections/%s/tasks/%s", collectionID, taskID))

	http.Redirect(w, r, taskURL.String(), http.StatusSeeOther)
}
