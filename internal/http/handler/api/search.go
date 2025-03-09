package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/pkg/errors"
)

type SearchResponse struct {
	Sections []*Section `json:"sections"`
	Sources  []string   `json:"sources"`
}

type Section struct {
	ID      model.SectionID `json:"id"`
	Source  string          `json:"source"`
	Content string          `json:"content"`
}

func (h *Handler) handleSearch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query().Get("query")

	res, err := h.doSearch(ctx, query)
	if err != nil {
		slog.ErrorContext(ctx, "could not search sections", slog.Any("error", errors.WithStack(err)))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", " ")

	if err := encoder.Encode(res); err != nil {
		slog.ErrorContext(ctx, "could not encode response", slog.Any("error", errors.WithStack(err)))
	}
}

func (h *Handler) doSearch(ctx context.Context, query string) (*SearchResponse, error) {
	searchResults, err := h.index.Search(ctx, query, &port.IndexSearchOptions{
		MaxResults: 3,
	})
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
			section, err := h.store.GetSectionBySourceAndID(ctx, r.Source, s)
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
