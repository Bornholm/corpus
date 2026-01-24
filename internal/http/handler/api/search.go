package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/bornholm/corpus/internal/core/service"
	"github.com/bornholm/corpus/internal/http/handler/webui/common"
	"github.com/bornholm/go-x/slogx"
	"github.com/pkg/errors"
)

type SearchResponse struct {
	Results []*SearchResult `json:"results"`
}

type SearchResult struct {
	Source   string                 `json:"source"`
	Sections []*SearchResultSection `json:"sections"`
}
type SearchResultSection struct {
	ID      model.SectionID `json:"id"`
	Content string          `json:"content"`
}

func (h *Handler) handleSearch(w http.ResponseWriter, r *http.Request) {
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

	rawSize := r.URL.Query().Get("size")
	var (
		size int64
	)
	if rawSize != "" {
		size, err = strconv.ParseInt(rawSize, 10, 64)
		if err != nil {
			slog.ErrorContext(ctx, "could not parse size parameter", slog.Any("error", errors.WithStack(err)))
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
	} else {
		size = 3
	}

	res, err := h.doSearch(ctx, query, collections, size)
	if err != nil {
		slog.ErrorContext(ctx, "could not search sections", slog.Any("error", errors.WithStack(err)))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", " ")

	w.Header().Set("Content-Type", "application/json")

	if err := encoder.Encode(res); err != nil {
		slog.ErrorContext(ctx, "could not encode response", slog.Any("error", errors.WithStack(err)))
	}
}

func (h *Handler) doSearch(ctx context.Context, query string, collections []model.CollectionID, size int64) (*SearchResponse, error) {
	slog.DebugContext(ctx, "executing search", slog.String("query", query), slog.Any("collections", collections), slog.Any("size", size))

	res := &SearchResponse{
		Results: []*SearchResult{},
	}

	if len(collections) == 0 {
		return res, nil
	}

	searchResults, err := h.documentManager.Search(ctx, query,
		service.WithDocumentManagerSearchCollections(collections...),
		service.WithDocumentManagerSearchMaxResults(int(size)),
	)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	for _, r := range searchResults {
		result := &SearchResult{
			Source:   r.Source.String(),
			Sections: []*SearchResultSection{},
		}

		for _, sectionID := range r.Sections {
			section, err := h.documentManager.DocumentStore.GetSectionByID(ctx, sectionID)
			if err != nil {
				if errors.Is(err, port.ErrNotFound) {
					continue
				}

				return nil, errors.WithStack(err)
			}

			content, err := section.Content()
			if err != nil {
				return nil, errors.WithStack(err)
			}

			result.Sections = append(result.Sections, &SearchResultSection{
				ID:      sectionID,
				Content: string(content),
			})
		}

		res.Results = append(res.Results, result)
	}

	return res, nil
}
