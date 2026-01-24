package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/core/service"
	"github.com/bornholm/corpus/internal/http/handler/webui/common"
	"github.com/bornholm/go-x/slogx"
	"github.com/pkg/errors"
)

type AskResponse struct {
	Response string                     `json:"response"`
	Contents map[model.SectionID]string `json:"contents"`
}

func (h *Handler) handleAsk(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query().Get("query")
	if query == "" {
		slog.ErrorContext(ctx, "missing query parameter")
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	collections, err := h.getSelectedCollectionsFromRequest(r)
	if err != nil {
		slog.ErrorContext(ctx, "could not retrieve collections from request", slogx.Error(err))
		var httpErr common.HTTPError
		if errors.As(err, &httpErr) {
			http.Error(w, http.StatusText(httpErr.StatusCode()), httpErr.StatusCode())
			return
		}

		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	res, err := h.doAsk(ctx, query, collections)
	if err != nil {
		var httpErr common.HTTPError
		if errors.As(err, &httpErr) {
			http.Error(w, httpErr.Error(), httpErr.StatusCode())
			return
		}

		slog.ErrorContext(ctx, "could not ask documents", slogx.Error(err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", " ")

	w.Header().Set("Content-Type", "application/json")

	if err := encoder.Encode(res); err != nil {
		slog.ErrorContext(ctx, "could not encode response", slogx.Error(err))
	}
}

func (h *Handler) doAsk(ctx context.Context, query string, collections []model.CollectionID) (*AskResponse, error) {
	slog.DebugContext(ctx, "executing ask query", slog.String("query", query), slog.Any("collections", collections))

	results, err := h.documentManager.Search(ctx, query,
		service.WithDocumentManagerSearchCollections(collections...),
	)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if len(results) == 0 {
		return nil, common.NewError("no results", "no matching results in document collection", http.StatusNoContent)
	}

	response, contents, err := h.documentManager.Ask(ctx, query, results)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	res := &AskResponse{
		Response: response,
		Contents: contents,
	}

	return res, nil
}
