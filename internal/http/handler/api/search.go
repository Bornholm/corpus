package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/bornholm/corpus/internal/core/service"
	"github.com/pkg/errors"
)

type SearchResponse struct {
	Sections []*Section `json:"sections"`
	Sources  []string   `json:"sources"`
}

type Section struct {
	ID          model.SectionID      `json:"id"`
	Source      string               `json:"source"`
	Content     string               `json:"content"`
	Collections []model.CollectionID `json:"collections,omitempty"`
}

func (h *Handler) handleSearch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query().Get("query")
	if query == "" {
		slog.ErrorContext(ctx, "missing query parameter")
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	collections := r.URL.Query()["collection"]

	rawSize := r.URL.Query().Get("size")
	var (
		size int64
		err  error
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

func (h *Handler) doSearch(ctx context.Context, query string, collections []string, size int64) (*SearchResponse, error) {
	slog.DebugContext(ctx, "executing search", slog.String("query", query), slog.Any("collections", collections), slog.Any("size", size))

	searchResults, err := h.documentManager.Search(ctx, query,
		service.WithDocumentManagerSearchCollections(collections...),
		service.WithDocumentManagerSearchMaxResults(int(size)),
	)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	sources := map[string]struct{}{}
	res := &SearchResponse{
		Sections: make([]*Section, 0),
		Sources:  []string{},
	}

	for _, r := range searchResults {
		sources[r.Source.String()] = struct{}{}
		sections := map[string]*Section{}

		for _, s := range r.Sections {
			section, err := h.documentManager.GetSectionBySourceAndID(ctx, r.Source, s)
			if err != nil {
				if errors.Is(err, port.ErrNotFound) {
					slog.ErrorContext(ctx, "could not find section", slog.String("source", r.Source.String()), slog.String("sectionID", string(s)))
					continue
				}

				return nil, errors.WithStack(err)
			}

			branch := branchToString(section.Branch())

			sections[branch] = &Section{
				ID:      section.ID(),
				Source:  r.Source.String(),
				Content: section.Content(),
				Collections: slices.Collect(func(yield func(model.CollectionID) bool) {
					for _, c := range section.Document().Collections() {
						if !yield(c.ID()) {
							return
						}
					}
				}),
			}
		}

		// Keep only ancestors
		for branch, section := range sections {
			if !hasAncestor(sections, branch) {
				res.Sections = append(res.Sections, section)
			}
		}

	}

	for s := range sources {
		res.Sources = append(res.Sources, s)
	}

	return res, nil
}

func branchToString(branch []model.SectionID) string {
	var sb strings.Builder
	for i, s := range branch {
		if i > 0 {
			sb.WriteString(".")
		}
		sb.WriteString(string(s))
	}
	return sb.String()
}

func hasAncestor(sections map[string]*Section, branch string) bool {
	for ancestor := range sections {
		if ancestor != branch && strings.HasPrefix(branch, ancestor) {
			return true
		}
	}

	return false
}
