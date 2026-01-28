package api

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"slices"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/core/port"
	"github.com/bornholm/corpus/internal/core/service"
	httpCtx "github.com/bornholm/corpus/internal/http/context"
	"github.com/pkg/errors"
)

type ListDocumentsResponse struct {
	Documents []DocumentHeader `json:"documents"`
	Total     int64            `json:"total"`
	Page      int              `json:"page"`
	Limit     int              `json:"limit"`
}

type DocumentHeader struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	ETag   string `json:"etag,omitempty"`
}

func (h *Handler) handleListDocuments(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	page := getQueryPage(query, 0)
	limit := getQueryLimit(query, 10)

	ctx := r.Context()

	opts := port.QueryDocumentsOptions{
		Page:       &page,
		Limit:      &limit,
		HeaderOnly: true,
	}

	if rawSource := query.Get("source"); rawSource != "" {
		source, err := url.Parse(rawSource)
		if err != nil {
			slog.ErrorContext(ctx, "invalid source parameter", slog.Any("error", errors.WithStack(err)))
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		opts.MatchingSource = source
	}

	user := httpCtx.User(ctx)

	readableDocuments, total, err := h.documentManager.DocumentStore.QueryUserReadableDocuments(ctx, user.ID(), opts)
	if err != nil {
		slog.ErrorContext(ctx, "could not query readable documents", slog.Any("error", errors.WithStack(err)))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	res := ListDocumentsResponse{
		Documents: make([]DocumentHeader, 0),
		Total:     total,
		Page:      page,
		Limit:     limit,
	}

	for _, d := range readableDocuments {
		res.Documents = append(res.Documents, DocumentHeader{
			ID:     string(d.ID()),
			ETag:   d.ETag(),
			Source: d.Source().String(),
		})
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", " ")

	w.Header().Set("Content-Type", "application/json")

	if err := encoder.Encode(res); err != nil {
		slog.ErrorContext(ctx, "could not encode response", slog.Any("error", errors.WithStack(err)))
	}
}

type GetDocumentResponse struct {
	Document Document `json:"document"`
}

type Document struct {
	DocumentHeader
	Sections    []string `json:"sections"`
	Collections []string `json:"collections"`
}

func (h *Handler) handleGetDocument(w http.ResponseWriter, r *http.Request) {
	documentID := model.DocumentID(r.PathValue("documentID"))

	ctx := r.Context()

	document, err := h.documentManager.DocumentStore.GetDocumentByID(ctx, documentID)
	if err != nil {
		if errors.Is(err, port.ErrNotFound) {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		slog.ErrorContext(ctx, "could not get document", slog.Any("error", errors.WithStack(err)))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	res := GetDocumentResponse{
		Document: Document{
			DocumentHeader: DocumentHeader{
				ID:     string(document.ID()),
				Source: document.Source().String(),
			},
			Collections: slices.Collect(func(yield func(string) bool) {
				for _, c := range document.Collections() {
					if !yield(string(c.ID())) {
						return
					}
				}
			}),
			Sections: slices.Collect(func(yield func(string) bool) {
				for _, s := range document.Sections() {
					if !yield(string(s.ID())) {
						return
					}
				}
			}),
		},
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", " ")

	w.Header().Set("Content-Type", "application/json")

	if err := encoder.Encode(res); err != nil {
		slog.ErrorContext(ctx, "could not encode response", slog.Any("error", errors.WithStack(err)))
	}
}

func (h *Handler) handleGetDocumentContent(w http.ResponseWriter, r *http.Request) {
	documentID := model.DocumentID(r.PathValue("documentID"))

	ctx := r.Context()

	document, err := h.documentManager.DocumentStore.GetDocumentByID(ctx, documentID)
	if err != nil {
		if errors.Is(err, port.ErrNotFound) {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		slog.ErrorContext(ctx, "could not get document", slog.Any("error", errors.WithStack(err)))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	content, err := document.Content()
	if err != nil {
		slog.ErrorContext(ctx, "could not retrieve document content", slog.Any("error", errors.WithStack(err)))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/markdown")

	if _, err := io.Copy(w, bytes.NewBuffer(content)); err != nil {
		slog.ErrorContext(ctx, "could not retrieve document content", slog.Any("error", errors.WithStack(err)))
		return
	}
}

func (h *Handler) handleReindexDocument(w http.ResponseWriter, r *http.Request) {
	documentID := model.DocumentID(r.PathValue("documentID"))

	ctx := r.Context()

	document, err := h.documentManager.DocumentStore.GetDocumentByID(ctx, documentID)
	if err != nil {
		if errors.Is(err, port.ErrNotFound) {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		slog.ErrorContext(ctx, "could not get document", slog.Any("error", errors.WithStack(err)))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	content, err := document.Content()
	if err != nil {
		slog.ErrorContext(ctx, "could not retrieve document content", slog.Any("error", errors.WithStack(err)))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	collections := slices.Collect(func(yield func(c model.CollectionID) bool) {
		for _, c := range document.Collections() {
			if !yield(c.ID()) {
				return
			}
		}
	})

	user := httpCtx.User(ctx)

	taskID, err := h.documentManager.IndexFile(
		ctx, user, "file.md", bytes.NewBuffer(content),
		service.WithDocumentManagerIndexFileCollections(collections...),
		service.WithDocumentManagerIndexFileSource(document.Source()),
	)

	baseURL := httpCtx.BaseURL(ctx)

	taskURL := baseURL.JoinPath("/api/v1/tasks", string(taskID))

	http.Redirect(w, r, taskURL.String(), http.StatusSeeOther)
}

type Section struct {
	ID       model.SectionID   `json:"id"`
	Branch   []model.SectionID `json:"branch"`
	Start    int               `json:"start"`
	End      int               `json:"end"`
	Parent   model.SectionID   `json:"parent,omitempty"`
	Sections []model.SectionID `json:"sections,omitempty"`
}

func (h *Handler) handleGetDocumentSection(w http.ResponseWriter, r *http.Request) {
	documentID := model.DocumentID(r.PathValue("documentID"))
	sectionID := model.SectionID(r.PathValue("sectionID"))

	ctx := r.Context()

	section, err := h.documentManager.DocumentStore.GetSectionByID(ctx, sectionID)
	if err != nil {
		if errors.Is(err, port.ErrNotFound) {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		slog.ErrorContext(ctx, "could not get section", slog.Any("error", errors.WithStack(err)))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if section.Document().ID() != documentID {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	res := Section{
		ID:     section.ID(),
		Branch: section.Branch(),
		Start:  section.Start(),
		End:    section.End(),
	}

	if parent := section.Parent(); parent != nil {
		res.Parent = parent.ID()
	}

	if sections := section.Sections(); sections != nil {
		res.Sections = slices.Collect(func(yield func(model.SectionID) bool) {
			for _, s := range sections {
				if !yield(s.ID()) {
					return
				}
			}
		})
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", " ")

	w.Header().Set("Content-Type", "application/json")

	if err := encoder.Encode(res); err != nil {
		slog.ErrorContext(ctx, "could not encode response", slog.Any("error", errors.WithStack(err)))
	}
}

func (h *Handler) handleGetSectionContent(w http.ResponseWriter, r *http.Request) {
	documentID := model.DocumentID(r.PathValue("documentID"))
	sectionID := model.SectionID(r.PathValue("sectionID"))

	ctx := r.Context()

	section, err := h.documentManager.DocumentStore.GetSectionByID(ctx, sectionID)
	if err != nil {
		if errors.Is(err, port.ErrNotFound) {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		slog.ErrorContext(ctx, "could not get section", slog.Any("error", errors.WithStack(err)))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if section.Document().ID() != documentID {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	content, err := section.Content()
	if err != nil {
		slog.ErrorContext(ctx, "could not retrieve section content", slog.Any("error", errors.WithStack(err)))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/markdown")

	if _, err := io.Copy(w, bytes.NewBuffer(content)); err != nil {
		slog.ErrorContext(ctx, "could not retrieve document content", slog.Any("error", errors.WithStack(err)))
		return
	}
}

func (h *Handler) handleDeleteDocument(w http.ResponseWriter, r *http.Request) {
	documentID := model.DocumentID(r.PathValue("documentID"))

	ctx := r.Context()

	err := h.documentManager.DocumentStore.DeleteDocumentByID(ctx, documentID)
	if err != nil {
		slog.ErrorContext(ctx, "could not delete document", slog.Any("error", errors.WithStack(err)))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
