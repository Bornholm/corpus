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

// --- Collection Share types ---

type CollectionShareResponse struct {
	ID           string `json:"id"`
	CollectionID string `json:"collectionId"`
	UserID       string `json:"userId"`
	UserEmail    string `json:"userEmail"`
	UserName     string `json:"userName"`
	Level        string `json:"level"`
}

type ListCollectionSharesResponse struct {
	Shares []CollectionShareResponse `json:"shares"`
}

type CreateCollectionShareRequest struct {
	UserID string `json:"userId"`
	Level  string `json:"level"`
}

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

	collection, err := h.documentManager.DocumentStore.GetCollectionByID(ctx, collectionID, false)
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

func (h *Handler) handleListCollectionShares(w http.ResponseWriter, r *http.Request) {
	collectionID := model.CollectionID(r.PathValue("collectionID"))
	ctx := r.Context()
	user := httpCtx.User(ctx)

	// Only owner can list shares
	collection, err := h.documentManager.DocumentStore.GetCollectionByID(ctx, collectionID, false)
	if err != nil {
		if errors.Is(err, port.ErrNotFound) {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		slog.ErrorContext(ctx, "could not get collection", slogx.Error(err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if collection.Owner().ID() != user.ID() {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}

	shares, err := h.documentManager.DocumentStore.GetCollectionShares(ctx, collectionID)
	if err != nil {
		slog.ErrorContext(ctx, "could not get collection shares", slogx.Error(err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	res := ListCollectionSharesResponse{
		Shares: make([]CollectionShareResponse, 0, len(shares)),
	}
	for _, s := range shares {
		res.Shares = append(res.Shares, CollectionShareResponse{
			ID:           string(s.ID()),
			CollectionID: string(s.CollectionID()),
			UserID:       string(s.SharedWith().ID()),
			UserEmail:    s.SharedWith().Email(),
			UserName:     s.SharedWith().DisplayName(),
			Level:        string(s.Level()),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", " ")
	if err := encoder.Encode(res); err != nil {
		slog.ErrorContext(ctx, "could not encode response", slogx.Error(err))
	}
}

func (h *Handler) handleCreateCollectionShare(w http.ResponseWriter, r *http.Request) {
	collectionID := model.CollectionID(r.PathValue("collectionID"))
	ctx := r.Context()
	user := httpCtx.User(ctx)

	// Only owner can create shares
	collection, err := h.documentManager.DocumentStore.GetCollectionByID(ctx, collectionID, false)
	if err != nil {
		if errors.Is(err, port.ErrNotFound) {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		slog.ErrorContext(ctx, "could not get collection", slogx.Error(err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if collection.Owner().ID() != user.ID() {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}

	var req CreateCollectionShareRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slog.ErrorContext(ctx, "could not decode request body", slogx.Error(err))
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	var level model.CollectionShareLevel
	switch req.Level {
	case string(model.CollectionShareLevelRead):
		level = model.CollectionShareLevelRead
	case string(model.CollectionShareLevelWrite):
		level = model.CollectionShareLevelWrite
	default:
		http.Error(w, "invalid level: must be 'read' or 'write'", http.StatusBadRequest)
		return
	}

	share, err := h.documentManager.DocumentStore.CreateCollectionShare(ctx, collectionID, model.UserID(req.UserID), level)
	if err != nil {
		slog.ErrorContext(ctx, "could not create collection share", slogx.Error(err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	res := CollectionShareResponse{
		ID:           string(share.ID()),
		CollectionID: string(share.CollectionID()),
		UserID:       string(share.SharedWith().ID()),
		UserEmail:    share.SharedWith().Email(),
		UserName:     share.SharedWith().DisplayName(),
		Level:        string(share.Level()),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", " ")
	if err := encoder.Encode(res); err != nil {
		slog.ErrorContext(ctx, "could not encode response", slogx.Error(err))
	}
}

func (h *Handler) handleDeleteCollectionShare(w http.ResponseWriter, r *http.Request) {
	collectionID := model.CollectionID(r.PathValue("collectionID"))
	shareID := model.CollectionShareID(r.PathValue("shareID"))
	ctx := r.Context()
	user := httpCtx.User(ctx)

	// Only owner can delete shares
	collection, err := h.documentManager.DocumentStore.GetCollectionByID(ctx, collectionID, false)
	if err != nil {
		if errors.Is(err, port.ErrNotFound) {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		slog.ErrorContext(ctx, "could not get collection", slogx.Error(err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if collection.Owner().ID() != user.ID() {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}

	if err := h.documentManager.DocumentStore.DeleteCollectionShare(ctx, shareID); err != nil {
		if errors.Is(err, port.ErrNotFound) {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		slog.ErrorContext(ctx, "could not delete collection share", slogx.Error(err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleDeleteCollection(w http.ResponseWriter, r *http.Request) {
	collectionID := model.CollectionID(r.PathValue("collectionID"))

	ctx := r.Context()

	// First check if collection exists
	_, err := h.documentManager.DocumentStore.GetCollectionByID(ctx, collectionID, false)
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
