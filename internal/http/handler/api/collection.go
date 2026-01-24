package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/bornholm/corpus/internal/core/model"
	"github.com/bornholm/corpus/internal/core/port"
	httpCtx "github.com/bornholm/corpus/internal/http/context"
	"github.com/bornholm/go-x/slogx"
	"github.com/pkg/errors"
)

type ListCollectionsResponse struct {
	Collections []CollectionHeader `json:"collections"`
	Total       int64              `json:"total"`
	Page        int                `json:"page"`
	Limit       int                `json:"limit"`
}

type CollectionHeader struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

type CollectionStats struct {
	TotalDocuments int64 `json:"totalDocuments"`
}

func (h *Handler) handleListCollections(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	page := getQueryPage(query, 0)
	limit := getQueryLimit(query, 10)

	ctx := r.Context()
	user := httpCtx.User(ctx)

	opts := port.QueryCollectionsOptions{
		Page:  &page,
		Limit: &limit,
	}

	readableCollections, total, err := h.documentManager.DocumentStore.QueryUserReadableCollections(ctx, user.ID(), opts)
	if err != nil {
		slog.ErrorContext(ctx, "could not query readable collections", slogx.Error(err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	res := ListCollectionsResponse{
		Collections: make([]CollectionHeader, 0),
		Total:       total,
		Page:        page,
		Limit:       limit,
	}

	for _, c := range readableCollections {
		res.Collections = append(res.Collections, CollectionHeader{
			ID:          string(c.ID()),
			Label:       c.Label(),
			Description: c.Description(),
		})
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", " ")

	w.Header().Set("Content-Type", "application/json")

	if err := encoder.Encode(res); err != nil {
		slog.ErrorContext(ctx, "could not encode response", slogx.Error(err))
	}
}

type GetCollectionResponse struct {
	Collection Collection `json:"collection"`
}

type Collection struct {
	CollectionHeader
	Stats *CollectionStats `json:"stats,omitempty"`
}

func (h *Handler) handleGetCollection(w http.ResponseWriter, r *http.Request) {
	collectionID := model.CollectionID(r.PathValue("collectionID"))

	ctx := r.Context()

	collection, err := h.documentManager.DocumentStore.GetCollectionByID(ctx, collectionID)
	if err != nil {
		if errors.Is(err, port.ErrNotFound) {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		slog.ErrorContext(ctx, "could not get collection", slogx.Error(err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	stats, err := h.documentManager.DocumentStore.GetCollectionStats(ctx, collectionID)
	if err != nil {
		slog.ErrorContext(ctx, "could not get collection stats", slogx.Error(err))
		// Don't fail the request if stats can't be retrieved
		stats = nil
	}

	res := GetCollectionResponse{
		Collection: Collection{
			CollectionHeader: CollectionHeader{
				ID:          string(collection.ID()),
				Label:       collection.Label(),
				Description: collection.Description(),
			},
			Stats: &CollectionStats{
				TotalDocuments: stats.TotalDocuments,
			},
		},
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", " ")

	w.Header().Set("Content-Type", "application/json")

	if err := encoder.Encode(res); err != nil {
		slog.ErrorContext(ctx, "could not encode response", slogx.Error(err))
	}
}

type UpdateCollectionRequest struct {
	Label       *string `json:"label,omitempty"`
	Description *string `json:"description,omitempty"`
}

func (h *Handler) handleUpdateCollection(w http.ResponseWriter, r *http.Request) {
	collectionID := model.CollectionID(r.PathValue("collectionID"))

	ctx := r.Context()

	var req UpdateCollectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.ErrorContext(ctx, "could not decode request body", slogx.Error(err))
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	updates := port.CollectionUpdates{
		Label:       req.Label,
		Description: req.Description,
	}

	collection, err := h.documentManager.DocumentStore.UpdateCollection(ctx, collectionID, updates)
	if err != nil {
		if errors.Is(err, port.ErrNotFound) {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		slog.ErrorContext(ctx, "could not update collection", slogx.Error(err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	res := GetCollectionResponse{
		Collection: Collection{
			CollectionHeader: CollectionHeader{
				ID:          string(collection.ID()),
				Label:       collection.Label(),
				Description: collection.Description(),
			},
		},
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", " ")

	w.Header().Set("Content-Type", "application/json")

	if err := encoder.Encode(res); err != nil {
		slog.ErrorContext(ctx, "could not encode response", slogx.Error(err))
	}
}

func (h *Handler) handleDeleteCollection(w http.ResponseWriter, r *http.Request) {
	collectionID := model.CollectionID(r.PathValue("collectionID"))

	ctx := r.Context()

	// First check if collection exists
	_, err := h.documentManager.DocumentStore.GetCollectionByID(ctx, collectionID)
	if err != nil {
		if errors.Is(err, port.ErrNotFound) {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		slog.ErrorContext(ctx, "could not get collection", slogx.Error(err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// Note: The actual deletion logic would need to be implemented in the DocumentStore
	// For now, we'll return a method not implemented error
	slog.ErrorContext(ctx, "collection deletion not implemented")
	http.Error(w, "Collection deletion not implemented", http.StatusNotImplemented)
}
