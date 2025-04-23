package api

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"

	"github.com/bornholm/corpus/internal/core/model"
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

	rawSource := r.FormValue("source")
	if rawSource != "" {
		source, err := url.Parse(rawSource)
		if err != nil {
			slog.ErrorContext(ctx, "could not parse source url", slog.Any("error", errors.WithStack(err)))
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		options = append(options, service.WithDocumentManagerIndexFileSource(source))
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

type indexResponse struct {
	Document jsonDocument `json:"document"`
}

type jsonDocument struct {
	ID          string           `json:"id"`
	Source      string           `json:"source"`
	Collections []jsonCollection `json:"collections,omitempty"`
	Sections    []jsonSection    `json:"sections"`
}

type jsonSection struct {
	ID       string        `json:"id"`
	Level    uint          `json:"level"`
	Sections []jsonSection `json:"sections,omitempty"`
}

type jsonCollection struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

func toIndexResponse(doc model.Document) *indexResponse {
	return &indexResponse{
		Document: jsonDocument{
			ID:          string(doc.ID()),
			Source:      doc.Source().String(),
			Collections: toJSONCollections(doc.Collections()),
			Sections:    toJSONSections(doc.Sections()),
		},
	}
}

func toJSONSections(sections []model.Section) []jsonSection {
	return slices.Collect(func(yield func(jsonSection) bool) {
		for _, s := range sections {
			if !yield(jsonSection{
				ID:       string(s.ID()),
				Level:    s.Level(),
				Sections: toJSONSections(s.Sections()),
			}) {
				return
			}
		}
	})
}

func toJSONCollections(collections []model.Collection) []jsonCollection {
	return slices.Collect(func(yield func(jsonCollection) bool) {
		for _, c := range collections {
			if !yield(jsonCollection{
				ID:          string(c.ID()),
				Description: c.Description(),
			}) {
				return
			}
		}
	})
}
